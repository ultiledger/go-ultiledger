package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// test validity of supplied key
func TestRandomOfferID(t *testing.T) {
	_, err := GetOfferID()
	assert.Equal(t, nil, err)
}
