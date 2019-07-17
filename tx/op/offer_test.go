package op

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db/memdb"
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestOfferOp(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)
	em := exchange.NewManager(memorydb, am)

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
	assert.Nil(t, err)
	assert.NotNil(t, trust)

	// create an offer
	buyAsset := &ultpb.Asset{AssetType: ultpb.AssetType_CUSTOM, AssetName: "COIN", Issuer: issuer}
	sellAsset := &ultpb.Asset{AssetType: ultpb.AssetType_NATIVE, AssetName: "ULU", Issuer: issuer}
	offerOp := Offer{
		AM:        am,
		EM:        em,
		AccountID: srcAccount,
		BuyAsset:  buyAsset,
		SellAsset: sellAsset,
		Amount:    1000,
		Price:     &ultpb.Price{Numerator: 2, Denominator: 3},
	}
	err = offerOp.Apply(memorytx)
	assert.Nil(t, err)

	// check the new offer
	offers, err := em.GetAccountOffers(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(offers))

	assert.Equal(t, srcAccount, offers[0].AccountID)
	assert.Equal(t, int64(1000), offers[0].Amount)
}
