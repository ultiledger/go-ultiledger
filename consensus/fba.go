package consensus

import (
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Federated Byzantine Agreement
type FBA struct{}

// watch for incoming Transaction
func (f *FBA) watchTx(txChan <-chan *pb.Transaction) {
	for {
		select {
		case tx := <-txChan:
			return
		}
	}
}
