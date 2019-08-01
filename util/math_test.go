package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntComparison(t *testing.T) {
	assert.Equal(t, 2, MaxInt(1, 2))
	assert.Equal(t, 2, MinInt(3, 2))
}

func TestInt64Comparison(t *testing.T) {
	assert.Equal(t, int64(2), MaxInt64(int64(1), int64(2)))
	assert.Equal(t, int64(2), MinInt64(int64(3), int64(2)))
}

func TestUint64Comparison(t *testing.T) {
	assert.Equal(t, uint64(2), MaxUint64(uint64(1), uint64(2)))
	assert.Equal(t, uint64(2), MinUint64(uint64(3), uint64(2)))
}
