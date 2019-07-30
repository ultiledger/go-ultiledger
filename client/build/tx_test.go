package build

import (
	"testing"

	"github.com/ultiledger/go-ultiledger/crypto"

	"github.com/stretchr/testify/assert"
)

func TestTx(t *testing.T) {
	// Create a random account.
	src, seed, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	dst, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := NewTx()

	// AccountID mutator.
	accID := &AccountID{AccountID: src}

	// Note mutator.
	n := &Note{Note: "SIMPLE NOTE"}

	// CreateAcount mutator.
	ca := &CreateAccount{
		AccountID: dst,
		Balance:   int64(1000),
	}

	// Payment mutator.
	p := &Payment{
		AccountID: dst,
		Amount:    int64(1000),
		Asset:     &Asset{AssetName: "ABC", Issuer: src, AssetType: NATIVE},
	}

	// Trust mutator.
	trust := &Trust{
		Asset: &Asset{AssetName: "XYZ", Issuer: dst, AssetType: CUSTOM},
		Limit: 100,
	}

	err = tx.Add(accID, n, ca, p, trust)
	assert.Nil(t, err)

	assert.Equal(t, tx.Tx.Fee, int64(3)*BaseFee)

	// Testing signing the tx.
	payload, signature, err := tx.Sign(seed)
	assert.Nil(t, err)

	result := crypto.Verify(src, signature, payload)
	assert.True(t, result)
}
