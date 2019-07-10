package op

import (
	"testing"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db/memdb"
	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

const (
	issuer = "Issuer"
)

func TestTrustOp(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)

	// create source account
	err := am.CreateAccount(memorydb, srcAccount, 1000000, signer, 2)
	assert.Nil(t, err)

	// create issuer account
	err = am.CreateAccount(memorydb, issuer, 1000000, signer, 3)
	assert.Nil(t, err)

	memorytx, _ := memorydb.Begin()

	// create trust op
	asset := &ultpb.Asset{
		AssetType: ultpb.AssetType_CUSTOM,
		AssetName: "COIN",
		Issuer:    issuer,
	}
	trustOp := Trust{
		AM:           am,
		SrcAccountID: srcAccount,
		Asset:        asset,
		Limit:        10000,
	}

	// test create new trust
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	// check src account entry count
	srcAcc, err := am.GetAccount(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, srcAcc.EntryCount, int32(1))

	// check created trust
	trust, err := am.GetTrust(memorytx, srcAccount, asset)
	assert.Equal(t, err, nil)
	assert.NotNil(t, trust)

	// lower the trust limit
	trustOp.Limit = 5000
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	// delete the trust
	trustOp.Limit = 0
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	trust, err = am.GetTrust(memorytx, srcAccount, asset)
	assert.Nil(t, err)
	assert.Nil(t, trust)
}
