package ledger

import (
	"crypto/sha256"
	"time"

	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/db"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

type LedgerState uint8

const (
	NOTSYNCED LedgerState = iota
	SYNCED
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
type ledgerManager struct {
	// db store and corresponding bucket
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	// ledger state
	ledgerState LedgerState
	// start timestamp of the manager
	startTime int64
	// timestamp of last ledger update
	lastUpdateTime int64
	// approximate number of ledgers under management
	approxLedgerCount int64
	// prev committed ledger header (for convenience)
	prevLedgerHeader *pb.LedgerHeader
	// current ledger header
	currLedgerHeader *pb.LedgerHeader
}

func NewLedgerManager(d db.DB, l *zap.SugaredLogger) *ledgerManager {
	lm := &ledgerManager{
		store:       d,
		bucket:      "LEDGERS",
		logger:      l,
		ledgerState: NOTSYNCED,
		startTime:   time.Now().Unix(),
	}
	err := lm.store.CreateBucket(lm.bucket)
	if err != nil {
		lm.logger.Fatal(err)
	}
	return lm
}

// Start the genesis ledger and initialize master account
func (lm *ledgerManager) CreateGenesisLedger() error {
	genesis := &pb.LedgerHeader{
		Version:       GenesisVersion,
		MaxTxListSize: GenesisMaxTxListSize,
		SeqNum:        GenesisSeqNum,
		TotalTokens:   GenesisTotalTokens,
		BaseFee:       GenesisBaseFee,
		BaseReserve:   GenesisBaseReserve,
		CloseTime:     time.Now().Unix(),
	}
	// compute hash of the ledger and save to db
	b, err := pb.Encode(genesis)
	if err != nil {
		return err
	}
	h := sha256.Sum256(b)
	lm.store.Set(lm.bucket, h[:], b)

	lm.currLedgerHeader = genesis

	return nil
}

// Append the committed transaction list to current ledger, the operation
// is only valid when the ledger manager in synced state.
func (lm *ledgerManager) AppendTxList(seq uint64, prevHash string, txHash string) {
	if lm.ledgerState != SYNCED {
		return
	}
	// check whether the sequence number is valid
	if lm.prevLedgerHeader.SeqNum-1 != seq {
		lm.logger.Warnw("ledger sequence number invalid", "prevSeqNum", lm.prevLedgerHeader.SeqNum, "currSeqNum", seq)
		return
	}

	// check whether the supplied prev tx list hash is identical
	// to the previous committed tx list hash
	if lm.prevLedgerHeader.TxListHash != prevHash {
		lm.logger.Fatalw("ledger prev tx hash mismatch", "committed", lm.prevLedgerHeader.TxListHash, "supplied", prevHash)
	}

	// TODO(bobonovski) save new ledger info
	h := &pb.LedgerHeader{SeqNum: seq, PrevLedgerHash: prevHash, TxListHash: txHash}
	lm.prevLedgerHeader = h
}
