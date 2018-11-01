package exchange

import (
	"math/big"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Compare two price
func ComparePrice(lhs *ultpb.Price, rhs *ultpb.Price) int {
	l := big.NewRat(lhs.Numerator, lhs.Denominator)
	r := big.NewRat(rhs.Numerator, rhs.Denominator)

	return l.Cmp(r)
}
