package op

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db/memdb"
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestOfferOp(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)
	em := exchange.NewManager(memorydb, am)

	// Create an account as the signer of the new account.
	signer, _, _ := crypto.GetAccountKeypair()

	// Create source account.
	srcAccount, _, _ := crypto.GetAccountKeypair()
	err := am.CreateAccount(memorydb, srcAccount, 1000000, signer, 2)
	assert.Nil(t, err)

	// Create asset issuer account.
	issuer, _, _ := crypto.GetAccountKeypair()
	err = am.CreateAccount(memorydb, issuer, 1000000, signer, 3)
	assert.Nil(t, err)

	memorytx, _ := memorydb.Begin()

	// Create trust of a custom asset for source account.
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

	// Apply the trust operator.
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	// Check src account entry count.
	srcAcc, err := am.GetAccount(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, srcAcc.EntryCount, int32(1))

	// Check the existance of the new trust.
	trust, err := am.GetTrust(memorytx, srcAccount, asset)
	assert.Nil(t, err)
	assert.NotNil(t, trust)

	// Create an offer to sell ULU for COIN.
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

	// As there are no offers in the exchange, we should leave an offer in
	// the exchange after the offer operator. Due to the irrationality of
	// the price, the offer amount should be the closest interger which is
	// divisible by the price. For amount = 1000, the number will be 999.
	offers, err := em.GetAccountOffers(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(offers))
	assert.Equal(t, srcAccount, offers[0].AccountID)
	assert.Equal(t, int64(999), offers[0].Amount)

	// After the creation of new offer, the source account should have a
	// selling liability of 999 of the native asset while the source trust
	// should have a buying liability of 666 of the custom asset.
	srcAcc, err = am.GetAccount(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, int64(999), srcAcc.Liability.Selling)

	trust, err = am.GetTrust(memorytx, srcAccount, asset)
	assert.Nil(t, err)
	assert.Equal(t, int64(666), trust.Liability.Buying)

	memorytx.Commit()
}
