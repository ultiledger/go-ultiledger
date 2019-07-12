package op

import (
	"errors"
	"fmt"

	pb "github.com/golang/protobuf/proto"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/util"
)

// Operation for managing offers.
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

	sellTrust, buyTrust, err := of.loadTrust(dt)
	if err != nil {
		return fmt.Errorf("load selling and buying trust failed: %v", err)
	}

	newOffer := true

	if of.OfferID != "" { // existing offer
		offer, err := of.EM.GetOffer(dt, of.SellAsset.AssetName, of.BuyAsset.AssetName, of.OfferID)
		if err != nil {
			return fmt.Errorf("get offer failed: %v", err)
		}
		if offer == nil {
			return errors.New("offer not exists")
		}

		// relese the liability of the offer
		err = of.updateLiability(dt, false)
		if err != nil {
			return fmt.Errorf("release offer liability failed: %v", err)
		}

		newOffer = false

		// delete the offer
		if of.Amount == 0 {
			err = of.EM.DeleteOffer(dt, of.SellAsset.AssetName, of.BuyAsset.AssetName, offer.OfferID)
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
		// create a offer id for the new offer
		offerID, err := crypto.GetOfferID()
		if err != nil {
			return fmt.Errorf("create offer id failed: %v", err)
		}
		of.OfferID = offerID
	}

	// create a new offer with the same offer id
	sellOffer := &ultpb.Offer{
		AccountID: of.AccountID,
		SellAsset: of.SellAsset,
		BuyAsset:  of.BuyAsset,
		Amount:    of.Amount,
		Price:     of.Price,
		OfferID:   of.OfferID,
		Passive:   of.Passive,
	}

	sellLimit, buyLimit, err := of.getOfferLimits(dt, acc, buyTrust, sellTrust, sellOffer, newOffer)
	if err != nil {
		return fmt.Errorf("get offer limits failed: %v", err)
	}

	if buyLimit == 0 {
		return errors.New("offer buying limit is zero")
	}
	// For test
	fmt.Println(sellLimit, buyLimit)

	return nil
}

// Get the buying and selling limits of the offer.
func (of *Offer) getOfferLimits(dt db.Tx, acc *ultpb.Account,
	buyTrust *ultpb.Trust, sellTrust *ultpb.Trust, offer *ultpb.Offer, newOffer bool) (int64, int64, error) {
	// temporarily increase entry count
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

	// get the limit of BuyAsset we can buy
	var buyLimit int64
	if of.BuyAsset.AssetType == ultpb.AssetType_NATIVE {
		buyLimit = of.AM.GetRestLimit(acc)
	} else {
		buyLimit = of.AM.GetTrustRestLimit(buyTrust)
	}

	bl, err := of.getBuyingLiability()
	if err != nil {
		return -1, -1, fmt.Errorf("get buying liability failed: %v", err)
	}
	if buyLimit < bl {
		return -1, -1, fmt.Errorf("out of buying limit")
	}

	// get the limit of SellAsset we can sell
	var sellLimit int64
	if of.SellAsset.AssetType == ultpb.AssetType_NATIVE {
		sellLimit = of.AM.GetBalance(acc)
	} else {
		sellLimit = of.AM.GetTrustBalance(sellTrust)
	}

	sl, err := of.getSellingLiability()
	if err != nil {
		return -1, -1, fmt.Errorf("get selling liability failed: %v", err)
	}
	if sellLimit < sl {
		return -1, -1, fmt.Errorf("account underfund")
	}

	// decrease entry count
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

	if buyLimit == 0 {
		return -1, -1, fmt.Errorf("account out of buying limit")
	}

	// get the sell limit for the offer
	sellLimit = util.MinInt64(sellLimit, offer.Amount)

	return sellLimit, buyLimit, nil
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

// Get buying liability of provided offer.
func (of *Offer) getBuyingLiability() (int64, error) {
	return 0, nil
}

// Get selling liability of provided offer.
func (of *Offer) getSellingLiability() (int64, error) {
	return 0, nil
}

// Update the liability of the account or the trust associated
// with the offer.
func (of *Offer) updateLiability(dt db.Tx, acquire bool) error {
	// compute buying liability
	bl, err := of.getBuyingLiability()
	if err != nil {
		return fmt.Errorf("get buying liability failed: %v", err)
	}
	if !acquire {
		bl = -bl
	}

	if of.BuyAsset.AssetType == ultpb.AssetType_NATIVE {
		acc, err := of.AM.GetAccount(dt, of.AccountID)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		// update account buying liability
		err = of.AM.UpdateLiability(acc, bl, true)
		if err != nil {
			return fmt.Errorf("update account buying liability failed: %v", err)
		}

		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		trust, err := of.AM.GetTrust(dt, of.AccountID, of.BuyAsset)
		if err != nil {
			return fmt.Errorf("get trust failed: %v", err)
		}

		// update trust buying liability
		err = of.AM.UpdateTrustLiability(trust, bl, true)
		if err != nil {
			return fmt.Errorf("update trust buying liability failed: %v", err)
		}

		err = of.AM.SaveTrust(dt, trust)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	}

	// compute selling liability
	sl, err := of.getSellingLiability()
	if err != nil {
		return fmt.Errorf("get selling liability failed: %v", err)
	}
	if acquire {
		sl = -sl
	}

	if of.SellAsset.AssetType == ultpb.AssetType_NATIVE {
		acc, err := of.AM.GetAccount(dt, of.AccountID)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		// update account buying liability
		err = of.AM.UpdateLiability(acc, sl, false)
		if err != nil {
			return fmt.Errorf("update account buying liability failed: %v", err)
		}

		err = of.AM.SaveAccount(dt, acc)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	} else {
		trust, err := of.AM.GetTrust(dt, of.AccountID, of.BuyAsset)
		if err != nil {
			return fmt.Errorf("get trust failed: %v", err)
		}

		// update trust buying liability
		err = of.AM.UpdateTrustLiability(trust, sl, true)
		if err != nil {
			return fmt.Errorf("update trust buying liability failed: %v", err)
		}

		err = of.AM.SaveTrust(dt, trust)
		if err != nil {
			return fmt.Errorf("save account failed: %v", err)
		}
	}

	return nil
}
