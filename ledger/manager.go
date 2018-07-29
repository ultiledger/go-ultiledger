package ledger

import (
	"time"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// ledger manager is responsible for all the operations on ledgers
type ledgerManager struct {
	// latest sequence number of ledger header
	latestSeqNum uint64
	// start timestamp of the manager
	startTime int64
	// timestamp of last ledger update
	lastUpdateTime int64
	// approximate number of ledgers under management
	approxLedgerCount int64
	// lastest ledger header (for convenience)
	latestLedgerHeader *ultpb.LedgerHeader
}

func NewLedgerManager() *ledgerManager {
	return &ledgerManager{startTime: time.Now().Unix()}
}
