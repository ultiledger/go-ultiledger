package tx

import (
	"fmt"
	"sync"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// TxHistory is used to hold unconfirmed transactions
type TxHistory struct {
	// maximum sequence number of the tx list
	MaxSeqNum uint64
	// total fees of the tx list
	TotalFees uint64
	// transaction map
	rw    sync.RWMutex
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

	th.rw.Lock()
	th.txMap[txKey] = tx
	th.rw.Unlock()

	return nil
}

// Get the flattened tx list
func (th *TxHistory) GetTxList() []*ultpb.Tx {
	var txList []*ultpb.Tx

	th.rw.RLock()
	for _, tx := range th.txMap {
		txList = append(txList, tx)
	}
	th.rw.RUnlock()

	return txList
}
