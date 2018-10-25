package op

import (
	"errors"
	"fmt"

	pb "github.com/golang/protobuf/proto"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Operation for creating a new offer
type Offer struct {
	AM           *account.Manager
	AccountID    string
	SellingAsset *ultpb.Asset
	BuyingAsset  *ultpb.Asset
	Amount       uint64
	Price        *ultpb.Price
	OfferID      uint64
	Passive      uint32
}

func (of *Offer) Apply(dt db.Tx) error {
	// check validity of assets
	if err := ValidateAsset(of.SellingAsset); err != nil {
		return fmt.Errorf("asset for selling is invalid: %v", err)
	}
	if err := ValidateAsset(of.BuyingAsset); err != nil {
		return fmt.Errorf("asset for buying is invalid: %v", err)
	}

	if pb.Equal(of.SellingAsset, of.BuyingAsset) {
		return errors.New("identical asset for offer")
	}

	if of.Price.Numerator == 0 || of.Price.Denominator == 0 {
		return errors.New("price is invalid")
	}

	if of.Amount == 0 && of.OfferID == 0 {
		return errors.New("offerID and amount are incompatible")
	}

	return nil
}
