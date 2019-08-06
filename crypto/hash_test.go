package crypto

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

var key string = "hello world!"

func TestSHA256Hash(t *testing.T) {
	digest := SHA256Hash([]byte(key))
	assert.Equal(t, len(digest), 44)
}

func TestSHA256HashUint64(t *testing.T) {
	num := SHA256HashUint64([]byte(key))
	assert.True(t, num < math.MaxUint64)
}
