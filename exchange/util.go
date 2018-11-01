package exchange

import (
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Compare two price
func ComparePrice(lhs *ultpb.Price, rhs *ultpb.Price) int {
	l := lhs.Numerator * rhs.Denominator
	r := lhs.Denominator * rhs.Numerator

	if l < r {
		return -1
	} else if l > r {
		return 1
	}

	return 0
}
