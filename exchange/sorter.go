package exchange

import "github.com/ultiledger/go-ultiledger/ultpb"

// Sort offers in ascending order by price.
type OfferSlice []*ultpb.Offer

func (os OfferSlice) Len() int {
	return len(os)
}

func (os OfferSlice) Less(i, j int) bool {
	cmp := ComparePrice(os[i].Price, os[j].Price)
	if cmp <= 0 {
		return true
	}
	return false
}

func (os OfferSlice) Swap(i, j int) {
	os[i], os[j] = os[j], os[i]
}
