package tx

import (
	"fmt"
	"testing"

	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

func TestTxHistory(t *testing.T) {
	txs := []*ultpb.Tx{
		&ultpb.Tx{AccountID: "A", SeqNum: uint64(1)},
		&ultpb.Tx{AccountID: "B", SeqNum: uint64(2)},
		&ultpb.Tx{AccountID: "C", SeqNum: uint64(3)},
		&ultpb.Tx{AccountID: "D", SeqNum: uint64(4)},
	}

	var keys []string

	txh := NewTxHistory()
	for i, tx := range txs {
		key := fmt.Sprintf("key-%d", i)
		txh.AddTx(key, tx)
		keys = append(keys, key)
	}

	assert.Equal(t, txh.Size(), 4)

	txh.DeleteTxList(keys)

	assert.Equal(t, txh.Size(), 0)
}
