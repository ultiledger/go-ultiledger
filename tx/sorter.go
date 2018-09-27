package tx

import "github.com/ultiledger/go-ultiledger/ultpb"

// Custom tx sort by sequence number
type TxSlice []*ultpb.Tx

func (ts TxSlice) Len() int {
	return len(ts)
}

func (ts TxSlice) Less(i, j int) bool {
	return ts[i].SequenceNumber < ts[j].SequenceNumber
}

func (ts TxSlice) Swap(i, j int) {
	ts[i], ts[j] = ts[j], ts[i]
}
