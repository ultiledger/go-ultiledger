package consensus

import (
	"errors"

	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

type TxHistory struct {
	// maximum sequence number of the tx list
	MaxSeqNum uint64
	// total fees of the tx list
	TotalFees uint64
	// list of transactions
	TxList []*pb.Tx
}

func NewTxHistory() *TxHistory {
	h := &TxHistory{
		MaxSeqNum: uint64(0),
		TotalFees: uint64(0),
		TxList:    make([]*pb.Tx, 0),
	}
	return h
}

func (th *TxHistory) AddTx(tx *pb.Tx) error {
	if tx.SequenceNumber < th.MaxSeqNum {
		return errors.New("tx sequence number is smaller than current largest sequence number")
	}
	th.TotalFees += tx.Fee
	th.TxList = append(th.TxList, tx)
	return nil
}
