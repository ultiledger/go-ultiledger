package ledger

import "github.com/ultiledger/go-ultiledger/ultpb"

// CloseInfo contains the information that the
// manager needs to close the current ledger.
type CloseInfo struct {
	// decree index
	Index uint64

	// encoded consensus value
	Value string

	// transaction set
	TxSet *ultpb.TxSet
}
