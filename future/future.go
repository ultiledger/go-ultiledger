package future

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// future for adding transaction
type Tx struct {
	Tx      *ultpb.Tx
	errChan chan error
}

func NewTx(tx *ultpb.Tx) *Tx {
	return &Tx{Tx: tx, errChan: make(chan error)}
}

func (t *Tx) Error() error {
	return <-t.errChan
}
