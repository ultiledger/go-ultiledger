package exchange

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestComparePrice(t *testing.T) {
	lhs := &ultpb.Price{
		Numerator:   uint64(3),
		Denominator: uint64(5),
	}
	rhs := &ultpb.Price{
		Numerator:   uint64(4),
		Denominator: uint64(7),
	}
	assert.Equal(t, 1, ComparePrice(lhs, rhs))

	lhs.Numerator = uint64(1)

	assert.Equal(t, -1, ComparePrice(lhs, rhs))

	lhs.Numerator = uint64(8)
	lhs.Denominator = uint64(14)

	assert.Equal(t, 0, ComparePrice(lhs, rhs))
}
