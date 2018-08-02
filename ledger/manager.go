package ledger

import (
	"time"

	"go.uber.org/zap"

	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

type LedgerState uint8

const (
	BOOTING LedgerState = iota
	SYNCED
)

// ledger manager is responsible for all the operations on ledgers
type ledgerManager struct {
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
}

func NewLedgerManager() *ledgerManager {
	return &ledgerManager{startTime: time.Now().Unix()}
}

// Append the committed transaction list to current ledger
func (lm *ledgerManager) AppendTxList(seq uint64, prevHash string, txHash string) {
	if lm.ledgerState != SYNCED {
		return
	}
	// check whether the sequence number is valid
	if lm.prevLedgerHeader.SeqNum-1 != seq {
		lm.logger.Warnw("ledger sequence number invalid", "prevSeqNum", lm.prevLedgerHeader.SeqNum, "currSeqNum", seq)
		return
	}

	// TODO(bobonovski) deal with older or newer ledger

	// check whether the supplied prev tx list hash is identical
	// to the previous committed tx list hash
	if lm.prevLedgerHeader.TxListHash != prevHash {
		lm.logger.Fatalw("ledger prev tx hash mismatch", "committed", lm.prevLedgerHeader.TxListHash, "supplied", prevHash)
	}

	// TODO(bobonovski) save new ledger info
	h := &pb.LedgerHeader{SeqNum: seq, PrevLedgerHash: prevHash, TxListHash: txHash}
	lm.prevLedgerHeader = h
}
