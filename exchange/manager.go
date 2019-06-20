package exchange

import (
	"fmt"
	"math/big"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Manager manages offers and fills asset exchange orders.
type Manager struct {
	AM     *account.Manager
	bucket string
	offers []*ultpb.Offer
}

// Fill order by loading existing offers from db and fill
// the order until the order is totally filled or there are
// not enough offers to use for filling the order.
func (m *Manager) FillOrder(dt db.Tx, o *Order) error {
	// The order is selling AssetX for AssetY, so we need to
	// load offers that sell AssetY for AssetX.
	err := m.loadOffers(dt, o.AssetY.AssetName, o.AssetX.AssetName)
	if err != nil {
		return fmt.Errorf("load offers failed: %v", err)
	}

	// Define the price threshold to be the reciprocal
	// price of the order price.
	threshold := &ultpb.Price{
		Numerator:   o.Price.Denominator,
		Denominator: o.Price.Numerator,
	}
	for _, offer := range m.offers {
		if ComparePrice(threshold, offer.Price) < 0 {
			break
		}
		m.fill(dt, o, offer)
		if o.Full == true {
			break
		}
	}

	return nil
}

// Fill the order with loaded offer. The order is selling AssetX
// for AssetY and the offer is selling AssetY for AssetX.
func (m *Manager) fill(dt db.Tx, o *Order, offer *ultpb.Offer) error {
	// Load seller account of the offer.
	acc, err := m.AM.GetAccount(dt, offer.AccountID)
	if err != nil {
		return fmt.Errorf("load seller account of offer failed: %v", err)
	}

	var assetXTrust, assetYTrust *ultpb.Trust
	if offer.SellAsset.AssetType != ultpb.AssetType_NATIVE {
		assetYTrust, err = m.AM.GetTrust(dt, offer.AccountID, offer.SellAsset)
		if err != nil {
			return fmt.Errorf("load sell asset trust for reciprocal offer failed: %v", err)
		}
	}
	if offer.BuyAsset.AssetType != ultpb.AssetType_NATIVE {
		assetXTrust, err = m.AM.GetTrust(dt, offer.AccountID, offer.BuyAsset)
		if err != nil {
			return fmt.Errorf("load buy asset trust for reciprocal offer failed: %v", err)
		}
	}

	// Maximum AssetY the offer can sell and maximum AssetX
	// the offer can buy.
	maxAssetY := m.getMaxToSell(acc, assetYTrust)
	maxAssetX := m.getMaxToBuy(acc, assetXTrust)

	// Exchange the asset with consistent rules.
	err = m.exchange(o, maxAssetY, maxAssetX, offer.Price, false)
	if err != nil {
		return fmt.Errorf("exchange assets failed: %v", err)
	}

	// Update the account balance or trust balance based on
	// information in order after exchange.
	if o.AssetYBought != 0 {
		if assetYTrust != nil {
			err = m.AM.AddTrustBalance(assetYTrust, o.AssetYBought)
			if err != nil {
				return fmt.Errorf("add trust balance failed: %v", err)
			}
			err = m.AM.SaveTrust(dt, assetYTrust)
			if err != nil {
				return fmt.Errorf("save trust failed: %v", err)
			}
		} else {
			err = m.AM.AddBalance(acc, o.AssetYBought)
			if err != nil {
				return fmt.Errorf("add account balance failed: %v", err)
			}
			err = m.AM.SaveAccount(dt, acc)
			if err != nil {
				return fmt.Errorf("save account failed: %v", err)
			}
		}
	}
	if o.AssetXSold != 0 {
		if assetXTrust != nil {
			err = m.AM.SubTrustBalance(assetXTrust, o.AssetXSold)
			if err != nil {
				return fmt.Errorf("substract trust balance failed: %v", err)
			}
			err = m.AM.SaveTrust(dt, assetYTrust)
			if err != nil {
				return fmt.Errorf("save trust failed: %v", err)
			}
		} else {
			err = m.AM.SubBalance(acc, o.AssetXSold)
			if err != nil {
				return fmt.Errorf("substract account balance failed: %v", err)
			}
			err = m.AM.SaveAccount(dt, acc)
			if err != nil {
				return fmt.Errorf("save account failed: %v", err)
			}
		}
	}

	return nil
}

// Exchange the assets for filling the order with selling limits
// of AssetY and buying limits of AssetX of the offer.
func (m *Manager) exchange(order *Order, maxAssetY int64, maxAssetX int64, offerPrice *ultpb.Price, checkError bool) error {
	// Note that the input price is the price of selling
	// AssetY for AssetX. We need to recover the order
	// price by exchanging the denominator and numberator.
	orderPrice := &ultpb.Price{
		Numerator:   offerPrice.Denominator,
		Denominator: offerPrice.Numerator,
	}

	// Scaled order value in terms of AssetY.
	orderValue := m.getOfferValue(order.MaxAssetX, order.MaxAssetY, orderPrice)
	// Scaled offer value in terms of AssetY.
	offerValue := m.getOfferValue(maxAssetY, maxAssetX, offerPrice)

	valueCmp := orderValue.Cmp(offerValue)

	var assetXSold, assetYBought int64

	// If valueCmp < 0, we should use orderValue to decide the
	// effective amount of AssetX and AssetY to exchange or we
	// should use offerValue.
	if valueCmp < 0 {
		if offerPrice.Numerator > offerPrice.Denominator { // AssetY is more valuable.
			assetYBought = DivideBigInt(orderValue, offerPrice.Numerator, RoundDown)
			assetXSold = DivideBigInt(MultiplyInt64(assetYBought, offerPrice.Numerator), offerPrice.Denominator, RoundUp)
		} else { // AssetX is more valuable.
			assetXSold = DivideBigInt(orderValue, offerPrice.Denominator, RoundDown)
			assetYBought = DivideBigInt(MultiplyInt64(assetXSold, offerPrice.Denominator), offerPrice.Numerator, RoundDown)
		}
	} else {
		if offerPrice.Numerator > offerPrice.Denominator {
			assetYBought = DivideBigInt(offerValue, offerPrice.Numerator, RoundDown)
			assetXSold = DivideBigInt(MultiplyInt64(assetYBought, offerPrice.Numerator), offerPrice.Denominator, RoundDown)
		} else {
			assetXSold = DivideBigInt(offerValue, offerPrice.Denominator, RoundDown)
			assetYBought = DivideBigInt(MultiplyInt64(assetXSold, offerPrice.Denominator), offerPrice.Numerator, RoundUp)
		}
	}

	// TODO(bobonovski) Check possible numerical errors during exchange

	order.AssetXSold = assetXSold
	order.AssetYBought = assetYBought
	// If valudCmp < 0, the current offer can fill the order
	if valueCmp < 0 {
		order.Full = true
	} else {
		order.Full = false
	}

	return nil
}

// Get the scaled value of the offer.
func (m *Manager) getOfferValue(maxToSell int64, maxToBuy int64, price *ultpb.Price) *big.Int {
	sellValue := MultiplyInt64(maxToSell, price.Numerator)
	buyValue := MultiplyInt64(maxToBuy, price.Denominator)

	cmp := sellValue.Cmp(buyValue)
	if cmp < 0 {
		return sellValue
	}
	return buyValue
}

// Get the maximum amount of asset the account or trust can sell.
func (m *Manager) getMaxToSell(acc *ultpb.Account, sellTrust *ultpb.Trust) int64 {
	var balance int64

	if sellTrust != nil && sellTrust.Authorized > 0 {
		balance = m.AM.GetTrustBalance(sellTrust)
		return balance
	}

	balance = m.AM.GetBalance(acc)

	return balance
}

// Get the maximum amount of asset the account or trust can buy.
func (m *Manager) getMaxToBuy(acc *ultpb.Account, buyTrust *ultpb.Trust) int64 {
	var balance int64

	if buyTrust != nil {
		balance = m.AM.GetTrustRestLimit(buyTrust)
		return balance
	}

	balance = m.AM.GetRestLimit(acc)

	return balance
}

// Load offers which sell lhsAsset and buy rhsAsset,
// the result offers are also filtered by whether their
// prices are cheaper than supplied price threshold.
func (m *Manager) loadOffers(dt db.Getter, lhsAsset string, rhsAsset string) error {
	prefix := []byte(lhsAsset + "_" + rhsAsset)
	bs, err := dt.GetAll(m.bucket, prefix)
	if err != nil {
		return fmt.Errorf("load offer list failed: %v", err)
	}

	var offers []*ultpb.Offer

	for i, _ := range bs {
		offer, err := ultpb.DecodeOffer(bs[i])
		if err != nil {
			return fmt.Errorf("decode offer failed: %v", err)
		}
		offers = append(offers, offer)
	}

	//TODO(bobonovski) Sort offers by price in increasing order

	m.offers = offers

	return nil
}
