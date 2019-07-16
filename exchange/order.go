package exchange

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Order contains the information for selling SellAsset for
// BuyAsset, every time an order is added to the exchange,
// we should search the existing offers that sell AssetY
// for AssetX to fill the order.
type Order struct {
	// account that created the order
	AccountID string
	// asset to sell
	SellAsset *ultpb.Asset
	// max amount of SellAsset we can sell
	MaxSellAsset int64
	// asset to buy
	BuyAsset *ultpb.Asset
	// max amount of BuyAsset we can buy
	MaxBuyAsset int64
	// price of SellAsset in terms of BuyAsset (price = BuyAsset / SellAsset)
	Price *ultpb.Price
	// amount of SellAsset we have sold after filling order
	SellAssetSold int64
	// amount of BuyAsset we have bought after filling order
	BuyAssetBought int64
	// filled offers
	FilledOffers []*ultpb.Offer
	// whether the order is fully filled
	Full bool
	// whether filter the offer with price constraint
	FilterPrice bool
}
