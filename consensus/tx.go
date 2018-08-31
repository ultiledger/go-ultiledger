package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/ultiledger/go-ultiledger/crypto"
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
	// list of hash of corresponding transactions
	TxHashList []string
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
func (th *TxHistory) AddTx(tx *ultpb.Tx, hash string) error {
	if tx.SequenceNumber < th.MaxSeqNum {
		return fmt.Errorf("tx seqnum mismatch: max %d, input %d", th.MaxSeqNum, tx.SequenceNumber)
	}
	th.TotalFees += tx.Fee
	th.TxList = append(th.TxList, tx)
	th.TxHashList = append(th.TxHashList, hash)
	return nil
}

// TxSet is used for making a transaction set for consensus
type TxSet struct {
	PrevLedgerHash string
	TxHashList     []string
}

// Compute the overall hash of transaction set
func (ts *TxSet) GetHash() (string, error) {
	// sort transaction hash list
	sort.Strings(ts.TxHashList)

	// append all the hash to buffer
	buf := bytes.NewBuffer(nil)
	b, err := hex.DecodeString(ts.PrevLedgerHash)
	if err != nil {
		return "", nil
	}
	buf.Write(b)

	for _, tx := range ts.TxHashList {
		txb, err := hex.DecodeString(tx)
		if err != nil {
			return "", err
		}
		buf.Write(txb)
	}

	hash := crypto.SHA256Hash(buf.Bytes())

	return hash, nil
}
