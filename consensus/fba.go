package consensus

import (
	"github.com/deckarep/golang-set"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Federated Byzantine Agreement
type FBA struct {
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	// consensus quorum
	quorum *pb.Quorum

	// latest voted slot
	latestSlotIdx uint64

	// vote round
	roundNum uint32

	// transactions waiting to be include in the ledger
	txSet  mapset.Set
	txList []*pb.Tx
	txChan <-chan *pb.Tx
}

func NewFBA(d db.DB, l *zap.SugaredLogger, txChan <-chan *pb.Tx) *FBA {
	f := &FBA{
		store:         d,
		bucket:        "FBA",
		logger:        l,
		latestSlotIdx: uint64(0),
		txSet:         mapset.NewSet(),
		txChan:        txChan,
	}
	err := f.store.CreateBucket(f.bucket)
	if err != nil {
		f.logger.Fatal(err)
	}
	return f
}

// watch for incoming Transaction
func (f *FBA) watchTx() {
	for {
		select {
		case tx := <-f.txChan:
			h, err := crypto.SHA256HashPb(tx)
			if err != nil {
				f.logger.Warnw("invalid transaction", "tx", tx)
				continue
			}
			if f.txSet.Contains(h) {
				f.logger.Warnw("duplicate transaction", "txHash", h)
				continue
			}
			f.txSet.Add(h)
			f.txList = append(f.txList, tx)
		}
	}
}

// Nominate computes a new nomination value based on hashes
// of previous and current transaction list.
func (f *FBA) Nominate(prevHash string, currHash string) (string, error) {
	f.roundNum += 1

	// TODO(bobonovski) prioritize validators for nomination

	return currHash, nil
}
