package consensus

import (
	"errors"

	"github.com/deckarep/golang-set"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/peer"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Engine is responsible for coordinating between upstream
// events and underlying consensus protocol
type Engine struct {
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	// peer manager for sending messages
	pm *peer.PeerManager

	// consensus quorum
	quorum *pb.Quorum

	// latest voted slot
	latestSlotIdx uint64

	// vote round
	roundNum uint32

	// consensus protocol
	cp *ucp

	// transactions waiting to be include in the ledger
	txSet  mapset.Set
	txList []*pb.Tx

	nominateChan chan string
}

func NewEngine(d db.DB, l *zap.SugaredLogger) *Engine {
	e := &Engine{
		store:         d,
		bucket:        "ENGINE",
		logger:        l,
		latestSlotIdx: uint64(0),
		cp:            newUCP(d, l),
		txSet:         mapset.NewSet(),
	}
	err := e.store.CreateBucket(e.bucket)
	if err != nil {
		e.logger.Fatal(err)
	}
	return e
}

func (e *Engine) Start(stopChan chan struct{}) {
	go func() {
		for {
			select {
			case <-stopChan:
				return
			}
		}
	}()
}

// Add transaction to internal pending set
func (e *Engine) AddTx(tx *pb.Tx) error {
	h, err := crypto.SHA256HashPb(tx)
	if err != nil {
		return err
	}

	if e.txSet.Contains(h) {
		return errors.New("duplicate transaction")
	}

	e.txSet.Add(h)
	e.txList = append(e.txList, tx)

	return nil
}
