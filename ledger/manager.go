package ledger

import (
	"errors"
	"fmt"
	"sort"
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

// TxResult represents the status of each tx,
// the tx is successfully applied if the err
// is nil or the error message will contains
// details about the error.
type TxResult struct {
	Err error
	Tx  *ultpb.Tx
}

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

	// internal channel for tx result
	txResultChan chan *TxResult

	// stop channel
	stopChan chan struct{}
}

func NewManager(d db.DB, am *account.Manager, tm *tx.Manager) *Manager {
	lm := &Manager{
		store:        d,
		bucket:       "LEDGERS",
		am:           am,
		tm:           tm,
		ledgerState:  LedgerStateNotSynced,
		startTime:    time.Now().Unix(),
		txResultChan: make(chan *TxResult, 10),
		stopChan:     make(chan struct{}),
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

// Ready returns transaction status with error messages
func (lm *Manager) Ready() <-chan *TxResult {
	return lm.txResultChan
}

// Receive externalized consensus value and do appropriate operations
func (lm *Manager) RecvExtVal(index uint64, value string, txset *ultpb.TxSet) error {
	log.Infow("recv ext value", "seq", index, "value", value, "prevhash", txset.PrevLedgerHash, "txcount", len(txset.TxList))

	switch lm.ledgerState {
	case LedgerStateSynced:
		if lm.NextLedgerHeaderSeq() == index+1 { // normal case
			if lm.CurrLedgerHeaderHash() == txset.PrevLedgerHash {
				// closeLedger
			} else {
				log.Fatalw("ledger inconsistent", "currhash", lm.CurrLedgerHeaderHash())
			}
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
	// TODO(bobonovski) support ledger version upgrades

	// sort tx by sequence number
	sort.Sort(TxSlice(txset.TxList))

	// group tx by account and txs of each account is sorted
	// by sequence number in increasing order
	accTxMap := make(map[string][]*ultpb.Tx)
	for _, tx := range txset.TxList {
		accTxMap[tx.AccountID] = append(accTxMap[tx.AccountID], tx)
	}

	// charge tx fees
	for id, txs := range accTxMap {
		acc, err := lm.am.GetAccount(id)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		// cache normal tx
		txList := make([]*ultpb.Tx, 0)

		i := 0
		for ; i < len(txs); i++ {
			// check validity of sequence number
			if acc.SequenceNumber > txs[i].SequenceNumber {
				lm.txResultChan <- &TxResult{
					Tx:  txs[i],
					Err: ErrInvalidSeqNum,
				}
			}
			// check sufficiency of balance
			if acc.Balance < txs[i].Fee {
				lm.txResultChan <- &TxResult{
					Tx:  txs[i],
					Err: ErrInsufficientForFee,
				}
			}
			acc.Balance -= txs[i].Fee
			acc.SequenceNumber = txs[i].SequenceNumber
			txList = append(txList, txs[i])
		}

		// shrink txs of accounts to only maintain normal tx
		accTxMap[id] = txList

		// update account
		err = lm.am.UpdateAccount(acc)
		if err != nil {
			return fmt.Errorf("update account %s failed: %v", id)
		}
	}

	// apply tx ops
	/*
		for id, txs := range accTxMap {
			acc, _ := lm.am.GetAccount(id) // we already checked the error in fee charge

			for _, tx := range txs {

			}
		}
	*/

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
