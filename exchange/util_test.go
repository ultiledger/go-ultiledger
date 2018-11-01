package exchange

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestComparePrice(t *testing.T) {
	lhs := &ultpb.Price{
		Numerator:   int64(3),
		Denominator: int64(5),
	}
	rhs := &ultpb.Price{
		Numerator:   int64(4),
		Denominator: int64(7),
	}
	assert.Equal(t, 1, ComparePrice(lhs, rhs))

	lhs.Numerator = int64(1)

	assert.Equal(t, -1, ComparePrice(lhs, rhs))

	lhs.Numerator = int64(8)
	lhs.Denominator = int64(14)

	assert.Equal(t, 0, ComparePrice(lhs, rhs))
}
