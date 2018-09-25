package consensus

import (
	"fmt"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// TxHistory is used to hold unconfirmed transactions
type TxHistory struct {
	// maximum sequence number of the tx list
	MaxSeqNum uint64
	// total fees of the tx list
	TotalFees uint64
	// list of transactions
	TxList []*ultpb.Tx
}

func NewTxHistory() *TxHistory {
	h := &TxHistory{
		MaxSeqNum: uint64(0),
		TotalFees: uint64(0),
		TxList:    make([]*ultpb.Tx, 0),
	}
	return h
}

// Add transaction to pending list, note that before
// adding any transaction, it should be checked against
// signature correctness, sufficient balance of account, etc.
func (th *TxHistory) AddTx(tx *ultpb.Tx) error {
	if tx.SequenceNumber < th.MaxSeqNum {
		return fmt.Errorf("tx seqnum mismatch: max %d, input %d", th.MaxSeqNum, tx.SequenceNumber)
	}
	th.TotalFees += tx.Fee
	th.TxList = append(th.TxList, tx)
	return nil
}

// Information about externalized value
type ExternalizeValue struct {
	Index uint64 // index of decree
	Value string // consensus value
}
