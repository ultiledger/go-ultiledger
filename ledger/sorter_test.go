package ledger

import (
	"sort"
	"testing"

	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

func TestTxSorter(t *testing.T) {
	txs := []*ultpb.Tx{
		&ultpb.Tx{AccountID: "A", SequenceNumber: uint64(2)},
		&ultpb.Tx{AccountID: "B", SequenceNumber: uint64(4)},
		&ultpb.Tx{AccountID: "C", SequenceNumber: uint64(3)},
		&ultpb.Tx{AccountID: "D", SequenceNumber: uint64(1)},
	}
	sort.Sort(TxSlice(txs))

	assert.Equal(t, *txs[0], ultpb.Tx{AccountID: "D", SequenceNumber: uint64(1)})
	assert.Equal(t, *txs[1], ultpb.Tx{AccountID: "A", SequenceNumber: uint64(2)})
	assert.Equal(t, *txs[2], ultpb.Tx{AccountID: "C", SequenceNumber: uint64(3)})
	assert.Equal(t, *txs[3], ultpb.Tx{AccountID: "B", SequenceNumber: uint64(4)})
}
