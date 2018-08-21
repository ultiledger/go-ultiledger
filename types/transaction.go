package types

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type TxHistory struct {
	// maximum sequence number of the tx list
	MaxSeqNum uint64
	// total fees of the tx list
	TotalFees uint64
	// list of transactions
	TxList []*pb.Tx
}
