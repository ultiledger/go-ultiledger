package consensus

import (
	"github.com/deckarep/golang-set"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Federated Byzantine Agreement
type fba struct {
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	// consensus quorum
	quorum *pb.Quorum

	// transactions waiting to be include in the ledger
	txSet  mapset.Set
	txList []*pb.Tx
	txChan <-chan *pb.Tx
}

func NewFBA(d db.DB, l *zap.SugaredLogger, txChan <-chan *pb.Tx) *fba {
	f := &fba{
		store:  d,
		bucket: "FBA",
		logger: l,
		txSet:  mapset.NewSet(),
		txChan: txChan,
	}
	err := f.store.CreateBucket(f.bucket)
	if err != nil {
		f.logger.Fatal(err)
	}
	return f
}

// watch for incoming Transaction
func (f *fba) watchTx() {
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
