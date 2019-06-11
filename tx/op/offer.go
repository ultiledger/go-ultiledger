package op

import (
	"errors"
	"fmt"

	pb "github.com/golang/protobuf/proto"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/util"
)

// Operation for managing offers
type Offer struct {
	AM        *account.Manager
	AccountID string
	SellAsset *ultpb.Asset
	BuyAsset  *ultpb.Asset
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

	sellTrust, buyTrust, err := of.loadTrust(dt)
	if err != nil {
		return fmt.Errorf("load selling and buying trust failed: %s", err)
	}

	newOffer := true

	if of.OfferID != "" {
		offer, err := of.AM.GetOffer(dt, of.OfferID)
		if err != nil {
			return fmt.Errorf("get offer failed: %v", err)
		}

		// TODO(bobonovski) release liability

		err = of.AM.DeleteOffer(dt, offer.OfferID)
		if err != nil {
			return fmt.Errorf("delete offer failed: %v", err)
		}

		newOffer = false

		// delete the offer
		if of.Amount == 0 {
			err = of.AM.SubEntryCount(acc, int32(1))
			if err != nil {
				return fmt.Errorf("decrease account entry account failed: %v", err)
			}
			err = of.AM.SaveAccount(dt, acc)
			if err != nil {
				return fmt.Errorf("save account failed: %v", err)
			}
			return nil
		}
	}

	// create a new offer
	sellOffer := &ultpb.Offer{
		AccountID: of.AccountID,
		SellAsset: of.SellAsset,
		BuyAsset:  of.BuyAsset,
		Amount:    of.Amount,
		Price:     of.Price,
		OfferID:   of.OfferID,
		Passive:   of.Passive,
	}

	// temporarily increase entry count
	if newOffer {
		err = of.AM.AddEntryCount(acc, int32(1))
		if err != nil {
			return fmt.Errorf("increase account entry account failed: %v", err)
		}
		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	}

	// get the limit of BuyAsset we can buy
	var buyLimit int64
	if of.BuyAsset.AssetType == ultpb.AssetType_NATIVE {
		buyLimit = of.AM.GetRestLimit(acc)
	} else {
		buyLimit = of.AM.GetTrustRestLimit(buyTrust)
	}

	if buyLimit < of.GetBuyLiability(sellOffer) {
		return fmt.Errorf("account out of buying limit")
	}

	// get the limit of SellAsset we can sell
	var sellLimit int64
	if of.SellAsset.AssetType == ultpb.AssetType_NATIVE {
		sellLimit = of.AM.GetBalance(acc)
	} else {
		sellLimit = of.AM.GetTrustBalance(sellTrust)
	}

	if sellLimit < of.GetSellLiability(sellOffer) {
		return fmt.Errorf("account underfund")
	}

	// decrease entry count
	if newOffer {
		err = of.AM.SubEntryCount(acc, int32(1))
		if err != nil {
			return fmt.Errorf("increase account entry account failed: %v", err)
		}
		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	}

	if buyLimit == 0 {
		return fmt.Errorf("account out of buying limit")
	}

	// get the sell limit for the offer
	// maxSellLimit := util.MinInt64(sellLimit, sellOffer.Amount)
	util.MinInt64(sellLimit, sellOffer.Amount)

	return nil
}

// Get buying liability of provided offer
func (of *Offer) GetBuyLiability(offer *ultpb.Offer) int64 {
	return 0
}

// Get selling liability of provided offer
func (of *Offer) GetSellLiability(offer *ultpb.Offer) int64 {
	return 0
}

// Load selling and buying trust
func (of *Offer) loadTrust(dt db.Tx) (*ultpb.Trust, *ultpb.Trust, error) {
	var sellTrust, buyTrust *ultpb.Trust
	var err error

	// load selling trust
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

	// load buying trust
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
	if err := ValidateAsset(of.SellAsset); err != nil {
		return fmt.Errorf("asset for selling is invalid: %v", err)
	}

	if err := ValidateAsset(of.BuyAsset); err != nil {
		return fmt.Errorf("asset for buying is invalid: %v", err)
	}

	if pb.Equal(of.SellAsset, of.BuyAsset) {
		return errors.New("identical asset for offer")
	}

	if of.Price.Numerator == 0 || of.Price.Denominator == 0 {
		return errors.New("price is invalid")
	}

	if of.Amount == 0 && of.OfferID == "" {
		return errors.New("offerID and amount are incompatible")
	}

	return nil
}
