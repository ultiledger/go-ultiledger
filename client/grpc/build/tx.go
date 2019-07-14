package build

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Tx serves as the main object for building an transaction.
type Tx struct {
	Tx *ultpb.Tx
}

func NewTx() *Tx {
	return &Tx{Tx: &ultpb.Tx{}}
}

// Add adds one or more mutators to the underlying transaction
// builder and if any of the mutation fails the method fails.
func (t *Tx) Add(ms ...TxMutator) error {
	var err error
	for _, m := range ms {
		err = m.Mutate(t.Tx)
		if err != nil {
			return err
		}
	}
	return nil
}
