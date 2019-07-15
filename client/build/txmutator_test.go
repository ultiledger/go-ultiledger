package build

import (
	"strings"
	"testing"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

func TestAccountMutator(t *testing.T) {
	// create random account
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	// test AccountID mutator with correct account key
	accID := AccountID{AccountID: pk}
	err = accID.Mutate(tx)
	assert.Nil(t, err)
	assert.Equal(t, tx.AccountID, accID.AccountID)

	// test AccountID mutator with incorrect account key
	accID = AccountID{AccountID: "InvalidID"}
	err = accID.Mutate(tx)
	assert.NotNil(t, err)
}

func TestNoteMutator(t *testing.T) {
	// create a long note
	var strs []string
	for i := 0; i < 1024; i++ {
		strs = append(strs, "X")
	}
	note := strings.Join(strs, "")

	tx := &ultpb.Tx{}

	// test Note mutator with invalid Note
	n := Note{Note: note}
	err := n.Mutate(tx)
	assert.NotNil(t, err)
}

func TestCreateAccountMutator(t *testing.T) {
	// create random account
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	ca := CreateAccount{
		AccountID: pk,
		Balance:   int64(1000),
	}
	err = ca.Mutate(tx)
	assert.Nil(t, err)

	ca.Balance = BaseFee - 1
	err = ca.Mutate(tx)
	assert.NotNil(t, err)
}

func TestPaymentMutator(t *testing.T) {
	// create random account
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	p := Payment{
		AccountID: pk,
		Amount:    int64(1000),
		Asset:     &Asset{AssetName: "ABC", Issuer: pk, AssetType: NATIVE},
	}
	err = p.Mutate(tx)
	assert.Nil(t, err)

	// test invalid payment amount
	p.Amount = int64(-1)
	err = p.Mutate(tx)
	assert.NotNil(t, err)
	p.Amount = int64(1000)

	// test invalid asset name
	p.Asset.AssetName = "ABCDE"
	err = p.Mutate(tx)
	assert.NotNil(t, err)
	p.Asset.AssetName = "ABC"

	// test invalid asset type
	p.Asset.AssetType = AssetType(2)
	err = p.Mutate(tx)
	assert.NotNil(t, err)
}

func TestTrustMutator(t *testing.T) {
	// create random account
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	trust := Trust{
		Asset: &Asset{AssetName: "ABC", Issuer: pk, AssetType: CUSTOM},
		Limit: 100,
	}
	err = trust.Mutate(tx)
	assert.Nil(t, err)

	// test invalid trust limit
	trust.Limit = int64(-1)
	err = trust.Mutate(tx)
	assert.NotNil(t, err)
}
