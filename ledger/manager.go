package ledger

import (
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type LedgerState uint8

const (
	LedgerStateSynced LedgerState = iota
	LedgerStateNotSynced
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

	// LRU cache for ledger headers
	headers *lru.Cache

	// ledger state
	ledgerState LedgerState

	// start timestamp of the manager
	startTime int64
	// timestamp of last ledger update
	lastUpdateTime int64

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
}

func NewManager(d db.DB) *Manager {
	lm := &Manager{
		store:       d,
		bucket:      "LEDGERS",
		ledgerState: LedgerStateNotSynced,
		startTime:   time.Now().Unix(),
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

// Start the genesis ledger and initialize master account
func (lm *Manager) CreateGenesisLedger() error {
	genesis := &ultpb.LedgerHeader{
		Version:       GenesisVersion,
		MaxTxListSize: GenesisMaxTxListSize,
		SeqNum:        GenesisSeqNum,
		TotalTokens:   GenesisTotalTokens,
		BaseFee:       GenesisBaseFee,
		BaseReserve:   GenesisBaseReserve,
		CloseTime:     time.Now().Unix(),
	}
	// compute hash of the ledger and save to db
	b, err := ultpb.Encode(genesis)
	if err != nil {
		return err
	}
	h := crypto.SHA256Hash(b)
	lm.store.Set(lm.bucket, []byte(h), b)

	lm.currLedgerHeader = genesis
	lm.currLedgerHeaderHash = h

	lm.ledgerHeaderCount += 1

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

// Append the committed transaction list to current ledger, the operation
// is only valid when the ledger manager in synced state.
func (lm *Manager) AppendTxList(seq uint64, prevHeaderHash string, txHash string) error {
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
		TxListHash:     txHash,
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

	return nil
}
