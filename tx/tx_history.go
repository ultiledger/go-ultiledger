package tx

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
	// transaction map
	txMap map[string]*ultpb.Tx
}

func NewTxHistory() *TxHistory {
	h := &TxHistory{
		MaxSeqNum: uint64(0),
		TotalFees: uint64(0),
		txMap:     make(map[string]*ultpb.Tx),
	}
	return h
}

// Add transaction to pending list, note that before
// adding any transaction, it should be checked against
// signature correctness, sufficient balance of account, etc.
func (th *TxHistory) AddTx(txKey string, tx *ultpb.Tx) error {
	if tx.SequenceNumber < th.MaxSeqNum {
		return fmt.Errorf("tx seqnum mismatch: max %d, input %d", th.MaxSeqNum, tx.SequenceNumber)
	}
	th.MaxSeqNum = tx.SequenceNumber
	th.TotalFees += tx.Fee

	th.txMap[txKey] = tx

	return nil
}

// Delete transactions and update fields
func (th *TxHistory) DeleteTxList(txKeys []string) {
	for _, txKey := range txKeys {
		if _, ok := th.txMap[txKey]; !ok {
			continue
		}
		delete(th.txMap, txKey)
	}

	// recalculate total fees and max seq
	maxseq := uint64(0)
	totalFees := uint64(0)
	for _, tx := range th.txMap {
		if tx.SequenceNumber > maxseq {
			maxseq = tx.SequenceNumber
		}
		totalFees += tx.Fee
	}
	th.MaxSeqNum = maxseq
	th.TotalFees = totalFees
}

// Get the flattened tx list
func (th *TxHistory) GetTxList() []*ultpb.Tx {
	var txList []*ultpb.Tx

	for _, tx := range th.txMap {
		txList = append(txList, tx)
	}

	return txList
}

// Get the size of internal tx map
func (th *TxHistory) Size() int {
	return len(th.txMap)
}
