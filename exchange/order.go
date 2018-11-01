package exchange

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Asset buying and selling order
type Order struct {
	// asset to sell
	SellAsset *ultpb.Asset
	// max amount of SellAsset we can sell
	MaxSellAmount uint64
	// asset to buy
	BuyAsset *ultpb.Asset
	// max amount of BuyAmount we can buy
	MaxBuyAmount uint64
	// price of SellAsset in terms of BuyAsset
	Price *ultpb.Price
	// defacto amount of SellAsset we have sold after filling order
	SoldAmount uint64
	// defacto amount of BuyAsset we have bought after filling order
	BoughtAmount uint64
}
