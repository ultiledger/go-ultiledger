package build

import (
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// We could import this constant from the ledger package
// but we define a constant value here for convenience.
var BaseFee = int64(1000)

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

	// add a fee mutator to compute the total fee
	fm := Fee{BaseFee: BaseFee}
	err = fm.Mutate(t.Tx)
	if err != nil {
		return err
	}

	// check the validity of tx
	if err := t.validate(); err != nil {
		return fmt.Errorf("tx is invaiid: %v", err)
	}

	return nil
}

func (t *Tx) validate() error {
	if t.Tx.AccountID == "" {
		return errors.New("empty account id")
	}
	if len(t.Tx.OpList) == 0 {
		return errors.New("empty op list")
	}
	return nil
}

// Sign the transaction data with supplied secret seed.
func (t *Tx) Sign(seed string) ([]byte, string, error) {
	if t.Tx == nil {
		return nil, "", ErrNilTx
	}

	// check the validity of seed
	seedKey, err := crypto.DecodeKey(seed)
	if err != nil {
		return nil, "", fmt.Errorf("decode seed key failed: %v", err)
	}
	if seedKey.Code != crypto.KeyTypeSeed {
		return nil, "", errors.New("incorrect seed key type")
	}

	// compute the signature
	payload, err := ultpb.Encode(t.Tx)
	if err != nil {
		return nil, "", fmt.Errorf("encode tx failed: %v", err)
	}
	signature, err := crypto.Sign(seed, payload)
	if err != nil {
		return nil, "", fmt.Errorf("sign the tx failed: %v", err)
	}

	return payload, signature, nil
}

// Get the tx key.
func (t *Tx) GetTxKey() (string, error) {
	if t.Tx == nil {
		return "", ErrNilTx
	}

	b, err := ultpb.Encode(t.Tx)
	if err != nil {
		return "", fmt.Errorf("encode tx failed: %v", err)
	}

	accKey := &crypto.ULTKey{
		Code: crypto.KeyTypeTx,
		Hash: crypto.SHA256HashBytes(b),
	}
	keyStr := crypto.EncodeKey(accKey)

	return keyStr, nil
}
