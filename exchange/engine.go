package exchange

import (
	"fmt"

	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Engine is responsible for managing offers and fill
// asset exchange orders
type Engine struct {
	bucket string
	offers []*ultpb.Offer
}

// Fill asset exchange order
func (e *Engine) FillOrder(dt db.Getter, o *Order) error {
	// load reciprocal offers of the order
	err := e.loadOffers(dt, o.BuyAsset.AssetName, o.SellAsset.AssetName)
	if err != nil {
		return fmt.Errorf("load reciprocal offers failed: %v", err)
	}

	// define the price threshold to be the reciprocal
	// price of input price
	threshold := &ultpb.Price{
		Numerator:   o.Price.Denominator,
		Denominator: o.Price.Numerator,
	}
	for _, offer := range e.offers {
		if ComparePrice(threshold, offer.Price) < 0 {
			break
		}

		e.fill(o, offer)
	}

	return nil
}

// Fill the order with specified reciprocal offer
func (e *Engine) fill(o *Order, offer *ultpb.Offer) {

}

// Load offers which sell lhsAsset and buy rhsAsset,
// the result offers are also filted by whether their
// prices are cheaper than supplied price threshold
func (e *Engine) loadOffers(dt db.Getter, lhsAsset string, rhsAsset string) error {
	key := []byte(lhsAsset + "_" + rhsAsset)
	b, err := dt.Get(e.bucket, key)
	if err != nil {
		return fmt.Errorf("load offer list failed: %v", err)
	}

	ol, err := ultpb.DecodeOfferList(b)
	if err != nil {
		return fmt.Errorf("decode offer list failed: %v", err)
	}

	e.offers = ol.Offers

	return nil
}
