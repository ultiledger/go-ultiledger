package build

import (
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrNilTx = errors.New("tx is nil")
)

// TxMutator defines the method which all the transaction
// related types should implement.
type TxMutator interface {
	Mutate(tx *ultpb.Tx) error
}

// AccountID sets the AccountID field in the tx.
type AccountID struct {
	AccountID string
}

func (a *AccountID) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}
	if a.AccountID == "" {
		return errors.New("empty account id")
	}

	// check whether the account id is valid ULTKey
	key, err := crypto.DecodeKey(a.AccountID)
	if err != nil {
		return fmt.Errorf("decode account id key failed: %v", err)
	}
	if key.Code != crypto.KeyTypeAccountID {
		return errors.New("incorrent account key type")
	}

	tx.AccountID = a.AccountID

	return nil
}

// Fee computes the total fee for the transaction.
type Fee struct {
	BaseFee int64
}

func (f *Fee) Mutate(tx *ultpb.Tx) error {
	if tx == nil {
		return ErrNilTx
	}
	if f.BaseFee < 0 {
		return errors.New("base fee is negative")
	}

	tx.Fee = f.BaseFee * int64(len(tx.OpList))

	return nil
}
