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

// Package exchange manages the distributed exchange.
package exchange

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/util"
)

// Manager manages offers and fills asset exchange orders.
type Manager struct {
	database    db.Database
	bucket      string
	offerBucket string
	nodeID      string

	AM     *account.Manager
	offers []*ultpb.Offer
}

func NewManager(database db.Database, am *account.Manager) *Manager {
	em := &Manager{
		database:    database,
		AM:          am,
		bucket:      "EXCHANGE",
		offerBucket: "OFFER",
	}
	err := em.database.NewBucket(em.bucket)
	if err != nil {
		log.Fatalf("create exchange bucket failed: %v", err)
	}
	err = em.database.NewBucket(em.offerBucket)
	if err != nil {
		log.Fatalf("create exchange offer bucket failed: %v", err)
	}
	return em
}

// Fill order by loading existing offers from db and fill
// the order until the order is totally filled or there are
// not enough offers to use for filling the order.
func (m *Manager) FillOrder(dt db.Tx, o *Order) error {
	// The order is selling SellAsset for BuyAsset, so we need to
	// load offers that sell BuyAsset for SellAsset.
	offers, err := m.loadOffers(dt, o.BuyAsset.AssetName, o.SellAsset.AssetName)
	if err != nil {
		return fmt.Errorf("load offers failed: %v", err)
	}

	// Define the price threshold to be the reciprocal
	// price of the order price.
	threshold := &ultpb.Price{
		Numerator:   o.Price.Denominator,
		Denominator: o.Price.Numerator,
	}
	for _, offer := range offers {
		if o.FilterPrice {
			if ComparePrice(threshold, offer.Price) < 0 {
				break
			}
		}
		// Cannot fill self offer.
		if offer.AccountID == o.AccountID {
			break
		}

		m.Fill(dt, o, offer)

		if !o.Full {
			o.FilledOffers = append(o.FilledOffers, offer)
		} else {
			break
		}
	}

	return nil
}

// Fill the order with loaded offer. The order is selling SellAsset
// for BuyAsset and the offer is selling BuyAsset for SellAsset.
func (m *Manager) Fill(dt db.Tx, o *Order, offer *ultpb.Offer) error {
	// Load seller account of the offer.
	acc, err := m.AM.GetAccount(dt, offer.AccountID)
	if err != nil {
		return fmt.Errorf("load seller account of offer failed: %v", err)
	}

	var sellAssetTrust, buyAssetTrust *ultpb.Trust
	if offer.SellAsset.AssetType != ultpb.AssetType_NATIVE {
		buyAssetTrust, err = m.AM.GetTrust(dt, offer.AccountID, offer.SellAsset)
		if err != nil {
			return fmt.Errorf("load sell asset trust for reciprocal offer failed: %v", err)
		}
	}
	if offer.BuyAsset.AssetType != ultpb.AssetType_NATIVE {
		sellAssetTrust, err = m.AM.GetTrust(dt, offer.AccountID, offer.BuyAsset)
		if err != nil {
			return fmt.Errorf("load buy asset trust for reciprocal offer failed: %v", err)
		}
	}

	// Release the liability of the offer.
	err = m.UpdateLiability(dt, offer, false)
	if err != nil {
		return fmt.Errorf("release liability failed: %v", err)
	}

	// Maximum BuyAsset the offer can sell and maximum SellAsset
	// the offer can buy.
	maxBuyAsset := m.GetMaxToSell(acc, buyAssetTrust)
	maxSellAsset := m.GetMaxToBuy(acc, sellAssetTrust)

	// Exchange the asset with consistent rules.
	err = m.Exchange(o, maxBuyAsset, maxSellAsset, offer.Price, false)
	if err != nil {
		return fmt.Errorf("exchange assets failed: %v", err)
	}

	// Update the account balance or trust balance based on
	// information in order after exchange.
	if o.BuyAssetBought != 0 {
		if buyAssetTrust != nil {
			err = m.AM.UpdateTrustBalance(buyAssetTrust, o.BuyAssetBought)
			if err != nil {
				return fmt.Errorf("add trust balance failed: %v", err)
			}
			err = m.AM.SaveTrust(dt, buyAssetTrust)
			if err != nil {
				return fmt.Errorf("save trust failed: %v", err)
			}
		} else {
			err = m.AM.UpdateBalance(acc, o.BuyAssetBought)
			if err != nil {
				return fmt.Errorf("add account balance failed: %v", err)
			}
			err = m.AM.SaveAccount(dt, acc)
			if err != nil {
				return fmt.Errorf("save account failed: %v", err)
			}
		}
	}
	if o.SellAssetSold != 0 {
		if sellAssetTrust != nil {
			err = m.AM.UpdateTrustBalance(sellAssetTrust, -o.SellAssetSold)
			if err != nil {
				return fmt.Errorf("substract trust balance failed: %v", err)
			}
			err = m.AM.SaveTrust(dt, buyAssetTrust)
			if err != nil {
				return fmt.Errorf("save trust failed: %v", err)
			}
		} else {
			err = m.AM.UpdateBalance(acc, -o.SellAssetSold)
			if err != nil {
				return fmt.Errorf("substract account balance failed: %v", err)
			}
			err = m.AM.SaveAccount(dt, acc)
			if err != nil {
				return fmt.Errorf("save account failed: %v", err)
			}
		}
	}

	// Update the current offer.
	if o.Full {
		offer.Amount -= o.BuyAssetBought
	} else {
		offer.Amount = 0
	}

	if offer.Amount == 0 {
		err = m.DeleteOffer(dt, offer)
		if err != nil {
			return fmt.Errorf("delete offer failed: $v", err)
		}

		err = m.AM.UpdateEntryCount(acc, int32(-1))
		if err != nil {
			return fmt.Errorf("update entry count failed: %v", err)
		}

		err = m.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		err = m.SaveOffer(dt, offer)
		if err != nil {
			return fmt.Errorf("save offer failed: %v", err)
		}
		err = m.UpdateLiability(dt, offer, true)
		if err != nil {
			return fmt.Errorf("acquire liability failed: %v", err)
		}
	}

	return nil
}

// Exchange the assets for filling the order with selling limits
// of BuyAsset and buying limits of SellAsset of the offer.
func (m *Manager) Exchange(order *Order, maxOfferBuyAsset int64, maxOfferSellAsset int64, offerPrice *ultpb.Price, checkError bool) error {
	orderPrice := order.Price

	// Scaled order value in terms of BuyAsset.
	orderValue := m.getOfferValue(order.MaxSellAsset, order.MaxBuyAsset, orderPrice)
	// Scaled offer value in terms of BuyAsset.
	offerValue := m.getOfferValue(maxOfferSellAsset, maxOfferBuyAsset, offerPrice)

	valueCmp := orderValue.Cmp(offerValue)

	// The amount of BuyAsset of order bought and the amount of
	// SellAsset of order sold.
	var sellAssetSold, buyAssetBought int64

	// If valueCmp < 0, we should use orderValue to decide the
	// effective amount of SellAsset and BuyAsset to exchange or we
	// should use offerValue.
	if valueCmp < 0 {
		if offerPrice.Numerator > offerPrice.Denominator { // SellAsset of offer is more valuable.
			buyAssetBought = DivideBigInt(orderValue, offerPrice.Numerator, RoundDown)
			sellAssetSold = DivideBigInt(MultiplyInt64(buyAssetBought, offerPrice.Numerator), offerPrice.Denominator, RoundUp)
		} else { // BuyAsset of offer is more valuable.
			sellAssetSold = DivideBigInt(orderValue, offerPrice.Denominator, RoundDown)
			buyAssetBought = DivideBigInt(MultiplyInt64(sellAssetSold, offerPrice.Denominator), offerPrice.Numerator, RoundDown)
		}
	} else {
		if offerPrice.Numerator > offerPrice.Denominator {
			buyAssetBought = DivideBigInt(offerValue, offerPrice.Numerator, RoundDown)
			sellAssetSold = DivideBigInt(MultiplyInt64(buyAssetBought, offerPrice.Numerator), offerPrice.Denominator, RoundDown)
		} else {
			sellAssetSold = DivideBigInt(offerValue, offerPrice.Denominator, RoundDown)
			buyAssetBought = DivideBigInt(MultiplyInt64(sellAssetSold, offerPrice.Denominator), offerPrice.Numerator, RoundUp)
		}
	}

	if checkError {
		// Check possible numerical errors during exchange
		if buyAssetBought < 0 || buyAssetBought > util.MinInt64(maxOfferSellAsset, order.MaxBuyAsset) {
			log.Fatal("order buy asset out of bounds")
		}
		if sellAssetSold < 0 || sellAssetSold > util.MinInt64(maxOfferBuyAsset, order.MaxSellAsset) {
			log.Fatal("order sell asset out of bounds")
		}
	}

	// Update order parameters.
	order.SellAssetSold += sellAssetSold
	order.MaxSellAsset -= sellAssetSold
	order.BuyAssetBought += buyAssetBought
	order.MaxBuyAsset -= buyAssetBought

	// Check whether the order is full after crossing the current offer.
	if valueCmp < 0 || order.MaxSellAsset == 0 || order.MaxBuyAsset == 0 {
		order.Full = true
	} else {
		order.Full = false
	}

	return nil
}

// Get the scaled value of the offer.
func (m *Manager) getOfferValue(maxToSell int64, maxToBuy int64, price *ultpb.Price) *big.Int {
	// Compare the value of maxToSell*price with maxToBuy.
	sellValue := MultiplyInt64(maxToSell, price.Numerator)
	buyValue := MultiplyInt64(maxToBuy, price.Denominator)

	cmp := sellValue.Cmp(buyValue)
	if cmp < 0 {
		return sellValue
	}
	return buyValue
}

// Get the maximum amount of asset the account or trust can sell.
func (m *Manager) GetMaxToSell(acc *ultpb.Account, sellTrust *ultpb.Trust) int64 {
	var balance int64

	if sellTrust != nil && sellTrust.Authorized > 0 {
		balance = m.AM.GetTrustBalance(sellTrust)
		return balance
	}

	balance = m.AM.GetBalance(acc)

	return balance
}

// Get the maximum amount of asset the account or trust can buy.
func (m *Manager) GetMaxToBuy(acc *ultpb.Account, buyTrust *ultpb.Trust) int64 {
	var balance int64

	if buyTrust != nil {
		balance = m.AM.GetTrustRestLimit(buyTrust)
		return balance
	}

	balance = m.AM.GetRestLimit(acc)

	return balance
}

// Load offers which sell sellAsset and buy buyAsset,
// the result offers are also filtered by whether their
// prices are cheaper than supplied price threshold.
func (m *Manager) loadOffers(getter db.Getter, sellAsset string, buyAsset string) ([]*ultpb.Offer, error) {
	prefix := []byte(sellAsset + "_" + buyAsset)
	bs, err := getter.GetAll(m.bucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("load offer list failed: %v", err)
	}

	var offers []*ultpb.Offer

	for i, _ := range bs {
		offer, err := ultpb.DecodeOffer(bs[i])
		if err != nil {
			return nil, fmt.Errorf("decode offer failed: %v", err)
		}
		offers = append(offers, offer)
	}

	// Sort offers in ascending order by price.
	sort.Sort(OfferSlice(offers))

	return offers, nil
}

// Get offer from database.
func (m *Manager) GetOffer(getter db.Getter, accountID, sellAsset, buyAsset, offerID string) (*ultpb.Offer, error) {
	key := m.GetOfferKey(accountID, offerID)
	b, err := getter.Get(m.offerBucket, key)
	if err != nil {
		return nil, fmt.Errorf("get offer from db failed: %v", err)
	}
	if b == nil {
		return nil, nil
	}

	offer, err := ultpb.DecodeOffer(b)
	if err != nil {
		return nil, fmt.Errorf("decode offer failed: %v", err)
	}

	if offer.OfferID != offerID {
		return nil, errors.New("offerID is incompatible")
	}
	if offer.SellAsset.AssetName != sellAsset {
		return nil, errors.New("sell asset incompatible")
	}
	if offer.BuyAsset.AssetName != buyAsset {
		return nil, errors.New("buy asset incompatible")
	}

	return offer, nil
}

// Get all the offers belong to an account.
func (m *Manager) GetAccountOffers(getter db.Getter, accountID string) ([]*ultpb.Offer, error) {
	prefix := []byte(accountID)
	bs, err := getter.GetAll(m.offerBucket, prefix)
	if err != nil {
		return nil, fmt.Errorf("load offer list failed: %v", err)
	}

	var offers []*ultpb.Offer
	for i, _ := range bs {
		offer, err := ultpb.DecodeOffer(bs[i])
		if err != nil {
			return nil, fmt.Errorf("decode offer failed: %v", err)
		}
		offers = append(offers, offer)
	}

	return offers, nil

}

// Update offer in database.
func (m *Manager) SaveOffer(putter db.Putter, offer *ultpb.Offer) error {
	offerb, err := ultpb.Encode(offer)
	if err != nil {
		return fmt.Errorf("encode offer failed: %v", err)
	}

	key := m.GetOfferKey(offer.SellAsset.AssetName, offer.BuyAsset.AssetName, offer.OfferID)
	err = putter.Put(m.bucket, key, offerb)
	if err != nil {
		return fmt.Errorf("save offer in db failed: %v", err)
	}

	key = m.GetOfferKey(offer.AccountID, offer.OfferID)
	err = putter.Put(m.offerBucket, key, offerb)
	if err != nil {
		return fmt.Errorf("save offer in db failed: %v", err)
	}

	return nil
}

// Delete offer in database.
func (m *Manager) DeleteOffer(deleter db.Deleter, offer *ultpb.Offer) error {
	if offer == nil {
		return nil
	}

	key := m.GetOfferKey(offer.SellAsset.AssetName, offer.BuyAsset.AssetName, offer.OfferID)
	err := deleter.Delete(m.bucket, key)
	if err != nil {
		return fmt.Errorf("deleter offer in db failed: %v", err)
	}

	key = m.GetOfferKey(offer.AccountID, offer.OfferID)
	err = deleter.Delete(m.offerBucket, key)
	if err != nil {
		return fmt.Errorf("deleter offer in offer db failed: %v", err)
	}

	return nil
}

// Helper function to generate the offer key.
func (m *Manager) GetOfferKey(names ...string) []byte {
	var buf []string
	for _, s := range names {
		buf = append(buf, s)
	}
	ks := strings.Join(buf, "_")
	return []byte(ks)
}

// Get the liability associated with an offer.
func (m *Manager) GetLiability(offer *ultpb.Offer, buying bool) (int64, error) {
	ord := &Order{
		MaxSellAsset: offer.Amount,
		MaxBuyAsset:  math.MaxInt64,
		Price:        offer.Price,
	}
	rp := &ultpb.Price{Denominator: offer.Price.Numerator, Numerator: offer.Price.Denominator}
	err := m.Exchange(ord, math.MaxInt64, math.MaxInt64, rp, false)
	if err != nil {
		return -1, fmt.Errorf("exchange assets failed: %v", err)
	}
	if buying {
		return ord.BuyAssetBought, nil
	}
	return ord.SellAssetSold, nil
}

// Update the account and trust liabilities associated with an offer.
func (m *Manager) UpdateLiability(dt db.Tx, offer *ultpb.Offer, acquire bool) error {
	// Compute buying liability.
	bl, err := m.GetLiability(offer, true)
	if err != nil {
		return fmt.Errorf("get buying liability failed: %v", err)
	}
	if !acquire {
		bl = -bl
	}

	if offer.BuyAsset.AssetType == ultpb.AssetType_NATIVE {
		acc, err := m.AM.GetAccount(dt, offer.AccountID)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		// Update account buying liability.
		err = m.AM.UpdateLiability(acc, bl, true)
		if err != nil {
			return fmt.Errorf("update account buying liability failed: %v", err)
		}

		err = m.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		trust, err := m.AM.GetTrust(dt, offer.AccountID, offer.BuyAsset)
		if err != nil {
			return fmt.Errorf("get trust failed: %v", err)
		}

		// Update trust buying liability.
		err = m.AM.UpdateTrustLiability(trust, bl, true)
		if err != nil {
			return fmt.Errorf("update trust buying liability failed: %v", err)
		}

		err = m.AM.SaveTrust(dt, trust)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	}

	// Compute selling liability.
	sl, err := m.GetLiability(offer, false)
	if err != nil {
		return fmt.Errorf("get selling liability failed: %v", err)
	}
	if !acquire {
		sl = -sl
	}

	if offer.SellAsset.AssetType == ultpb.AssetType_NATIVE {
		acc, err := m.AM.GetAccount(dt, offer.AccountID)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		// Update account selling liability.
		err = m.AM.UpdateLiability(acc, sl, false)
		if err != nil {
			return fmt.Errorf("update account selling liability failed: %v", err)
		}

		err = m.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		trust, err := m.AM.GetTrust(dt, offer.AccountID, offer.BuyAsset)
		if err != nil {
			return fmt.Errorf("get trust failed: %v", err)
		}

		// Update trust selling liability.
		err = m.AM.UpdateTrustLiability(trust, sl, false)
		if err != nil {
			return fmt.Errorf("update trust buying liability failed: %v", err)
		}

		err = m.AM.SaveTrust(dt, trust)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	}

	return nil
}
