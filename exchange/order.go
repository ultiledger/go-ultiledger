package exchange

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Order contains the information for selling AssetX for
// AssetY, every time an order is added to the exchange,
// we should search the existing offers that sell AssetY
// for AssetX to fill the order.
type Order struct {
	// asset to sell
	AssetX *ultpb.Asset
	// max amount of AssetX we can sell
	MaxAssetX int64
	// asset to buy
	AssetY *ultpb.Asset
	// max amount of AssetY we can buy
	MaxAssetY int64
	// price of AssetX in terms of AssetY (price = AssetY / AssetX)
	Price *ultpb.Price
	// amount of ETH we have sold after filling order
	AssetXSold int64
	// amount of BTC we have bought after filling order
	AssetYBought int64
	// whether the order is fully filled
	Full bool
}
