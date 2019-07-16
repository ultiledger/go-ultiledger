package exchange

import (
	"sort"
	"testing"

	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

func TestOfferSorter(t *testing.T) {
	offers := []*ultpb.Offer{
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(3)}},
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(4)}},
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(5)}},
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(4)}},
	}
	sort.Sort(OfferSlice(offers))

	assert.Equal(t, *offers[0], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(5)}})
	assert.Equal(t, *offers[1], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(4)}})
	assert.Equal(t, *offers[2], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(4)}})
	assert.Equal(t, *offers[3], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(3)}})
}
