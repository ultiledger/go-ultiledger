package consensus

import (
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Federated Byzantine Agreement
type fba struct {
	txList []*pb.Tx
	txChan <-chan *pb.Tx
}

func NewFBA(txChan <-chan *pb.Tx) *fba {
	return &fba{txChan: txChan}
}

// watch for incoming Transaction
func (f *fba) watchTx() {
	for {
		select {
		case tx := <-f.txChan:
			f.txList = append(f.txList, tx)
			return
		}
	}
}
