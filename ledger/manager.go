package ledger

import (
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/tx"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// LedgerState represents the current states of ledger,
// the ledger is synced if it complies with the states
// received from peers or we need to trigger catch up
// process to sync with the newest states.
type LedgerState uint8

const (
	LedgerStateSynced LedgerState = iota
	LedgerStateSyncing
	LedgerStateNotSynced
)

var (
	ErrUnknownLedgerState = errors.New("unknown ledger state")
	ErrInsufficientForFee = errors.New("insufficient balance for fee")
	ErrInsufficientForTx  = errors.New("insufficient balance for tx")
	ErrInvalidSeqNum      = errors.New("invalid sequence number")
)

var (
	GenesisVersion       = uint32(1)
	GenesisMaxTxListSize = uint32(100)
	GenesisSeqNum        = uint64(1)
	GenesisTotalTokens   = uint64(4500000000000000000)
	GenesisBaseFee       = uint64(1000)
	GenesisBaseReserve   = uint64(1000000000)
)

// ledger manager is responsible for all the operations on ledgers
type Manager struct {
	// db store and corresponding bucket
	store  db.DB
	bucket string

	// account manager
	am *account.Manager

	// tx manager
	tm *tx.Manager

	// LRU cache for ledger headers
	headers *lru.Cache

	// ledger state
	ledgerState LedgerState

	// start timestamp of the manager
	startTime int64
	// timestamp of last ledger close
	lastCloseTime int64

	// the number of ledgers processed
	ledgerHeaderCount int64

	// previous committed ledger header
	prevLedgerHeader *ultpb.LedgerHeader
	// hash of previous committed ledger header
	prevLedgerHeaderHash string

	// current latest committed ledger header
	currLedgerHeader *ultpb.LedgerHeader
	// hash of current latest committed ledger header
	currLedgerHeaderHash string

	// stop channel
	stopChan chan struct{}
}

func NewManager(d db.DB, am *account.Manager, tm *tx.Manager) *Manager {
	lm := &Manager{
		store:       d,
		bucket:      "LEDGERS",
		am:          am,
		tm:          tm,
		ledgerState: LedgerStateNotSynced,
		startTime:   time.Now().Unix(),
		stopChan:    make(chan struct{}),
	}
	cache, err := lru.New(1000)
	if err != nil {
		log.Fatalf("create ledger manager LRU cache failed: %v", err)
	}
	lm.headers = cache
	err = lm.store.CreateBucket(lm.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", lm.bucket, err)
	}
	return lm
}

// Start the ledger manager
func (lm *Manager) Start() {}

// Stop the ledger manager
func (lm *Manager) Stop() {
	close(lm.stopChan)
}

// Start the genesis ledger and initialize master account
func (lm *Manager) CreateGenesisLedger() error {
	// close genesis ledger
	err := lm.advanceLedger(GenesisSeqNum, "", "", "")
	if err != nil {
		return fmt.Errorf("advance ledger failed: %v", err)
	}
	return nil
}

// Get the current latest ledger header
func (lm *Manager) CurrLedgerHeader() *ultpb.LedgerHeader {
	// current ledger header should always exist
	if lm.currLedgerHeader == nil {
		log.Fatal("current ledger header is nil")
	}
	return lm.currLedgerHeader
}

// Get the hash of current latest ledger header
func (lm *Manager) CurrLedgerHeaderHash() string {
	// current ledger header hash should always exist
	if lm.currLedgerHeaderHash == "" {
		log.Fatal("current ledger header hash is empty")
	}
	return lm.currLedgerHeaderHash
}

// Get the sequence number of next ledger header
func (lm *Manager) NextLedgerHeaderSeq() uint64 {
	// current ledger header should always exist
	if lm.currLedgerHeader == nil {
		log.Fatal("current ledger header is nil")
	}
	return lm.currLedgerHeader.SeqNum + 1
}

// Receive externalized consensus value and do appropriate operations
func (lm *Manager) RecvExtVal(index uint64, value string, txset *ultpb.TxSet) error {
	log.Infow("recv ext value", "seq", index, "value", value, "prevhash", txset.PrevLedgerHash, "txcount", len(txset.TxList))

	switch lm.ledgerState {
	case LedgerStateSynced:
		if lm.NextLedgerHeaderSeq() == index+1 { // normal case
			if lm.CurrLedgerHeaderHash() == txset.PrevLedgerHash {
				err := lm.closeLedger(index, value, txset)
				if err != nil {
					return fmt.Errorf("close ledger failed: %v", err)
				}
			} else {
				log.Fatalw("ledger inconsistent", "currhash", lm.CurrLedgerHeaderHash())
			}

			// update ledger state
			lm.ledgerState = LedgerStateSynced
		} else if index <= lm.NextLedgerHeaderSeq() { // old case
			log.Warnw("received value is old", "nextseq", lm.NextLedgerHeaderSeq())
		} else { // new case
			// init catch up
		}
	case LedgerStateSyncing:
	case LedgerStateNotSynced:
		// TODO(bobonovski) catch up
	}

	return nil
}

// CloseLedger closes current ledger with new consensus value
func (lm *Manager) closeLedger(index uint64, value string, txset *ultpb.TxSet) error {
	// apply transactions
	err := lm.tm.ApplyTxList(txset.TxList)
	if err != nil {
		return fmt.Errorf("apply tx list failed: %v", err)
	}

	txsetHash, err := ultpb.GetTxSetHash(txset)
	if err != nil {
		return fmt.Errorf("get txset hash failed: %v", err)
	}

	err = lm.advanceLedger(index, txset.PrevLedgerHash, txsetHash, value)
	if err != nil {
		return fmt.Errorf("advance ledger failed: %v", err)
	}

	return nil
}

// Append the committed transaction list to current ledger, the operation
// is only valid when the ledger manager in synced state.
func (lm *Manager) advanceLedger(seq uint64, prevHeaderHash string, txHash string, cv string) error {
	if lm.ledgerState != LedgerStateSynced {
		return errors.New("ledger is not synced")
	}

	// Check whether the sequence number is valid
	if lm.currLedgerHeader.SeqNum+1 != seq {
		return fmt.Errorf("ledger seqnum mismatch: expect %d, got %d", lm.currLedgerHeader.SeqNum+1, seq)
	}

	// Check whether the supplied prev ledger header hash is identical
	// to the current ledger header hash. It should never happen that
	// these two values are not identical which means some fork happened.
	if lm.currLedgerHeaderHash != prevHeaderHash {
		log.Fatalw("ledger tx header hash mismatch", "curr", lm.currLedgerHeaderHash, "prev", prevHeaderHash)
	}

	header := &ultpb.LedgerHeader{
		SeqNum:         seq,
		PrevLedgerHash: prevHeaderHash,
		TxSetHash:      txHash,
		ConsensusValue: cv,
		// global configs below
		Version:       GenesisVersion,
		MaxTxListSize: GenesisMaxTxListSize,
		TotalTokens:   GenesisTotalTokens,
		BaseFee:       GenesisBaseFee,
		BaseReserve:   GenesisBaseReserve,
		CloseTime:     time.Now().Unix(),
	}
	b, err := ultpb.Encode(header)
	if err != nil {
		return fmt.Errorf("encode ledger header failed: %v", err)
	}
	h := crypto.SHA256Hash(b)
	lm.store.Set(lm.bucket, []byte(h), b)

	// advance current ledger header
	lm.prevLedgerHeader = lm.currLedgerHeader
	lm.prevLedgerHeaderHash = lm.currLedgerHeaderHash
	lm.currLedgerHeader = header
	lm.currLedgerHeaderHash = h

	lm.ledgerHeaderCount += 1

	return nil
}
