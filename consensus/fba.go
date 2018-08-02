package consensus

import (
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Federated Byzantine Agreement
type fba struct {
	txChan <-chan *pb.Tx
}

func NewFBA(txChan *pb.Tx) {
	return &fba{txChan: txChan}
}

// watch for incoming Transaction
func (f *FBA) watchTx() {
	for {
		select {
		case tx := <-f.txChan:
			return
		}
	}
}
