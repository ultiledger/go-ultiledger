// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package op

import (
	"errors"
	"fmt"
	"math"

	pb "github.com/golang/protobuf/proto"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/util"
)

// Operator for managing offers.
type Offer struct {
	AM        *account.Manager
	EM        *exchange.Manager
	AccountID string
	BuyAsset  *ultpb.Asset
	SellAsset *ultpb.Asset
	Amount    int64
	Price     *ultpb.Price
	OfferID   string
	Passive   int32
}

func (of *Offer) Apply(dt db.Tx) error {
	if err := of.validate(); err != nil {
		return fmt.Errorf("validate offer failed: %v", err)
	}

	acc, err := of.AM.GetAccount(dt, of.AccountID)
	if err != nil {
		return fmt.Errorf("get account failed: %v", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}

	sellTrust, buyTrust, err := of.loadTrust(dt)
	if err != nil {
		return fmt.Errorf("load selling and buying trust failed: %v", err)
	}

	var newOffer bool

	if of.OfferID != "" { // Changing an existing offer.
		offer, err := of.EM.GetOffer(dt, of.AccountID, of.SellAsset.AssetName, of.BuyAsset.AssetName, of.OfferID)
		if err != nil {
			return fmt.Errorf("get offer failed: %v", err)
		}
		if offer == nil {
			return errors.New("offer not exists")
		}

		// Relese the liability of the offer.
		err = of.EM.UpdateLiability(dt, offer, false)
		if err != nil {
			return fmt.Errorf("release offer liability failed: %v", err)
		}

		newOffer = false

		// Delete the offer if the amount is zero.
		if of.Amount == 0 {
			err = of.EM.DeleteOffer(dt, offer)
			if err != nil {
				return fmt.Errorf("delete offer failed: %v", err)
			}

			err = of.AM.UpdateEntryCount(acc, int32(-1))
			if err != nil {
				return fmt.Errorf("decrease account entry account failed: %v", err)
			}

			err = of.AM.SaveAccount(dt, acc)
			if err != nil {
				return fmt.Errorf("save account failed: %v", err)
			}
			return nil
		}
	} else {
		if of.Amount == 0 {
			return nil
		}
		newOffer = true
	}

	// Create a new offer.
	sellOffer := &ultpb.Offer{
		AccountID: of.AccountID,
		SellAsset: of.SellAsset,
		BuyAsset:  of.BuyAsset,
		Amount:    of.Amount,
		Price:     of.Price,
		OfferID:   of.OfferID,
		Passive:   of.Passive,
	}

	sellLimit, buyLimit, err := of.getOfferLimits(dt, acc, sellTrust, buyTrust, sellOffer, newOffer)
	if err != nil {
		return fmt.Errorf("get offer limits failed: %v", err)
	}

	if buyLimit == 0 {
		return errors.New("offer buying limit is zero")
	}

	// Fill the order in exchange.
	order := &exchange.Order{
		AccountID:    of.AccountID,
		SellAsset:    of.SellAsset,
		MaxSellAsset: sellLimit,
		BuyAsset:     of.BuyAsset,
		MaxBuyAsset:  buyLimit,
		Price:        of.Price,
		FilterPrice:  true,
	}
	err = of.EM.FillOrder(dt, order)
	if err != nil {
		return fmt.Errorf("fill order failed: %v", err)
	}

	if order.BuyAssetBought > 0 {
		err = of.updateBalances(dt, acc, sellTrust, buyTrust, order.SellAssetSold, order.BuyAssetBought)
		if err != nil {
			return fmt.Errorf("update account and trust balances failed: %v", err)
		}
	}

	if order.Full {
		sellOffer.Amount = 0
	} else {
		// Adjust the offer amount based on filled order.
		sellOffer.Amount = sellLimit - order.SellAssetSold
		sl := util.MinInt64(sellOffer.Amount, of.EM.GetMaxToSell(acc, sellTrust))
		bl := of.EM.GetMaxToBuy(acc, buyTrust)
		ord := &exchange.Order{
			SellAsset:    sellOffer.SellAsset,
			MaxSellAsset: sl,
			BuyAsset:     sellOffer.BuyAsset,
			MaxBuyAsset:  bl,
			Price:        sellOffer.Price,
		}
		rp := &ultpb.Price{Numerator: ord.Price.Denominator, Denominator: ord.Price.Numerator}
		err = of.EM.Exchange(ord, math.MaxInt64, math.MaxInt64, rp, false)
		if err != nil {
			return fmt.Errorf("exchange assets failed: %v", err)
		}
		sellOffer.Amount = ord.SellAssetSold
	}

	if sellOffer.Amount > 0 {
		if newOffer {
			// Create a offer id for the new offer.
			offerID, err := crypto.GetOfferID()
			if err != nil {
				return fmt.Errorf("create offer id failed: %v", err)
			}
			sellOffer.OfferID = offerID
		}
		err = of.EM.SaveOffer(dt, sellOffer)
		if err != nil {
			return fmt.Errorf("save offer failed: %v", err)
		}

		// Acquire the liability of the offer.
		err = of.EM.UpdateLiability(dt, sellOffer, true)
		if err != nil {
			return fmt.Errorf("acquire offer liability failed: %v", err)
		}
	}

	return nil
}

// Update the balances of buy and sell assets.
func (of *Offer) updateBalances(dt db.Tx, acc *ultpb.Account,
	sellTrust, buyTrust *ultpb.Trust, sellAssetSold, buyAssetBought int64) error {
	var err error

	// Update the balance for buy asset.
	if of.BuyAsset.AssetType == ultpb.AssetType_NATIVE {
		err = of.AM.UpdateBalance(acc, buyAssetBought)
		if err != nil {
			return fmt.Errorf("update account balance failed: %v", err)
		}
		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		err = of.AM.UpdateTrustBalance(buyTrust, buyAssetBought)
		if err != nil {
			return fmt.Errorf("update trust balance failed: %v", err)
		}
		err = of.AM.SaveTrust(dt, buyTrust)
		if err != nil {
			return fmt.Errorf("save trust failed: %v", err)
		}
	}

	// Update the balance for sell asset.
	if of.SellAsset.AssetType == ultpb.AssetType_NATIVE {
		err = of.AM.UpdateBalance(acc, -sellAssetSold)
		if err != nil {
			return fmt.Errorf("update account balance failed: %v", err)
		}
		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		err = of.AM.UpdateTrustBalance(sellTrust, -sellAssetSold)
		if err != nil {
			return fmt.Errorf("update trust balance failed: %v", err)
		}
		err = of.AM.SaveTrust(dt, sellTrust)
		if err != nil {
			return fmt.Errorf("save trust failed: %v", err)
		}
	}

	return nil
}

// Get the buying and selling limits of the offer.
func (of *Offer) getOfferLimits(dt db.Tx, acc *ultpb.Account,
	sellTrust, buyTrust *ultpb.Trust, offer *ultpb.Offer, newOffer bool) (int64, int64, error) {
	// Temporarily increase entry count.
	var err error
	if newOffer {
		err = of.AM.UpdateEntryCount(acc, int32(1))
		if err != nil {
			return -1, -1, fmt.Errorf("increase account entry account failed: %v", err)
		}
		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return -1, -1, fmt.Errorf("save account failed: %v", err)
		}
	}

	// Get the limit of BuyAsset we can buy.
	var buyLimit int64
	if of.BuyAsset.AssetType == ultpb.AssetType_NATIVE {
		buyLimit = of.AM.GetRestLimit(acc)
	} else {
		buyLimit = of.AM.GetTrustRestLimit(buyTrust)
	}

	bl, err := of.EM.GetLiability(offer, true)
	if err != nil {
		return -1, -1, fmt.Errorf("get buying liability failed: %v", err)
	}
	if buyLimit < bl {
		return -1, -1, fmt.Errorf("out of buying limit")
	}

	// Get the limit of SellAsset we can sell.
	var sellLimit int64
	if of.SellAsset.AssetType == ultpb.AssetType_NATIVE {
		sellLimit = of.AM.GetBalance(acc)
	} else {
		sellLimit = of.AM.GetTrustBalance(sellTrust)
	}

	sl, err := of.EM.GetLiability(offer, false)
	if err != nil {
		return -1, -1, fmt.Errorf("get selling liability failed: %v", err)
	}
	if sellLimit < sl {
		return -1, -1, fmt.Errorf("account underfund")
	}

	// Decrease entry count.
	if newOffer {
		err = of.AM.UpdateEntryCount(acc, int32(-1))
		if err != nil {
			return -1, -1, fmt.Errorf("increase account entry account failed: %v", err)
		}
		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return -1, -1, fmt.Errorf("save account failed: %v", err)
		}
	}

	// Get the sell limit for the offer.
	sellLimit = util.MinInt64(sellLimit, offer.Amount)

	return sellLimit, buyLimit, nil
}

// Load selling and buying trust.
func (of *Offer) loadTrust(dt db.Tx) (*ultpb.Trust, *ultpb.Trust, error) {
	var sellTrust, buyTrust *ultpb.Trust
	var err error

	// Load selling trust.
	if of.SellAsset.AssetType != ultpb.AssetType_NATIVE {
		sellTrust, err = of.AM.GetTrust(dt, of.AccountID, of.SellAsset)
		if err != nil {
			return nil, nil, fmt.Errorf("get selling trust failed: %v", err)
		}

		_, err = of.AM.GetAccount(dt, of.SellAsset.Issuer)
		if err != nil {
			return nil, nil, fmt.Errorf("get selling asset issuer failed: %v", err)
		}

		if sellTrust.Balance == 0 {
			return nil, nil, errors.New("selling trust is underfund")
		}

		if sellTrust.Authorized == 0 {
			return nil, nil, errors.New("selling trust is not authorized")
		}
	}

	// Load buying trust.
	if of.BuyAsset.AssetType != ultpb.AssetType_NATIVE {
		buyTrust, err = of.AM.GetTrust(dt, of.AccountID, of.BuyAsset)
		if err != nil {
			return nil, nil, fmt.Errorf("get buying trust failed: %v", err)
		}

		_, err = of.AM.GetAccount(dt, of.BuyAsset.Issuer)
		if err != nil {
			return nil, nil, fmt.Errorf("get buying asset issuer failed: %v", err)
		}

		if buyTrust.Authorized == 0 {
			return nil, nil, errors.New("buying trust is not authorized")
		}
	}

	return sellTrust, buyTrust, nil
}

func (of *Offer) validate() error {
	if err := validateAsset(of.SellAsset); err != nil {
		return fmt.Errorf("asset for selling is invalid: %v", err)
	}

	if err := validateAsset(of.BuyAsset); err != nil {
		return fmt.Errorf("asset for buying is invalid: %v", err)
	}

	if pb.Equal(of.SellAsset, of.BuyAsset) {
		return errors.New("identical asset for offer")
	}

	if of.Price.Numerator == 0 || of.Price.Denominator == 0 {
		return errors.New("price is invalid")
	}

	if of.Amount == 0 && of.OfferID == "" {
		return errors.New("amount of new offer is zero")
	}

	return nil
}
