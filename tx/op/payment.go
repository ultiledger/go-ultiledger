package op

import (
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrInvalidPaymentAmount = errors.New("invalid payment amount")
	ErrInvalidAccountID     = errors.New("invalid accountID")
)

func ValidateAsset(asset *ultpb.Asset) error {
	if asset == nil {
		return errors.New("asset is nil")
	}
	if asset.AssetType == ultpb.AssetType_NATIVE {
		return nil
	}
	if len(asset.AssetName) <= 0 || len(asset.AssetName) >= 4 {
		return errors.New("invalid asset name")
	}
	return nil
}

// Peer to peer payment in specified asset.
type Payment struct {
	AM           *account.Manager
	SrcAccountID string
	DstAccountID string
	Asset        *ultpb.Asset
	Amount       uint64
}

func (p *Payment) Apply() error {
	if err := ValidateAsset(p.Asset); err != nil {
		return fmt.Errorf("validate payment asset failed: %v", err)
	}
	if p.Amount == 0 {
		return ErrInvalidPaymentAmount
	}
	if p.SrcAccountID == "" || p.DstAccountID == "" {
		return ErrInvalidAccountID
	}

	// Payment is equivalent to path payment with one hop,
	// so we construct a path payment to reuse the logic.
	pp := &PathPayment{
		AM:           p.AM,
		SrcAccountID: p.SrcAccountID,
		SrcAsset:     p.Asset,
		SrcAmount:    p.Amount,
		DstAccountID: p.DstAccountID,
		DstAsset:     p.Asset,
		DstAmount:    p.Amount,
	}
	if err := pp.Apply(); err != nil {
		return err
	}

	return nil
}

// Path payment from source asset to destination asset.
type PathPayment struct {
	AM           *account.Manager
	SrcAccountID string
	SrcAsset     *ultpb.Asset
	SrcAmount    uint64
	DstAccountID string
	DstAsset     *ultpb.Asset
	DstAmount    uint64
	Path         []*ultpb.Asset
}

func (pp *PathPayment) Apply() error {
	if err := ValidateAsset(pp.SrcAsset); err != nil {
		return fmt.Errorf("validate src payment asset failed: %v", err)
	}
	if err := ValidateAsset(pp.DstAsset); err != nil {
		return fmt.Errorf("validate dst payment asset failed: %v", err)
	}
	for _, a := range pp.Path {
		if err := ValidateAsset(a); err != nil {
			return fmt.Errorf("validate path payment asset failed: %v", err)
		}
	}
	if pp.SrcAccountID == "" || pp.DstAccountID == "" {
		return ErrInvalidAccountID
	}
	if pp.SrcAmount == 0 || pp.DstAmount == 0 {
		return ErrInvalidPaymentAmount
	}

	// build asset path
	var path []*ultpb.Asset
	path = append(path, pp.SrcAsset)
	path = append(path, pp.Path...)

	// TODO(bobonovski) use DB batch

	dstAccount, err := pp.AM.GetAccount(pp.DstAccountID)
	if err != nil {
		return fmt.Errorf("get dst account failed: %v", err)
	}

	if pp.DstAsset.AssetType == ultpb.AssetType_NATIVE {
		if err = pp.AM.AddBalance(dstAccount, pp.DstAmount); err != nil {
			return err
		}
	} else {

	}

	return nil
}
