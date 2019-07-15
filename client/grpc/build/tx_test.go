package build

import (
	"testing"

	"github.com/ultiledger/go-ultiledger/crypto"

	"github.com/stretchr/testify/assert"
)

func TestTx(t *testing.T) {
	// create random account
	src, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	dst, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := NewTx()

	// AccountID mutator
	accID := &AccountID{AccountID: src}

	// Note mutator
	n := &Note{Note: "SIMPLE NOTE"}

	// CreateAcount mutator
	ca := &CreateAccount{
		AccountID: dst,
		Balance:   int64(1000),
	}

	// Payment mutator
	p := &Payment{
		AccountID: dst,
		Amount:    int64(1000),
		Asset:     &Asset{AssetName: "ABC", Issuer: src, AssetType: NATIVE},
	}

	// Trust mutator
	trust := &Trust{
		Asset: &Asset{AssetName: "XYZ", Issuer: dst, AssetType: CUSTOM},
		Limit: 100,
	}

	err = tx.Add(accID, n, ca, p, trust)
	assert.Nil(t, err)

	assert.Equal(t, tx.Tx.Fee, int64(3)*BaseFee)
}
