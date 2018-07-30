package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var key string = "hello world!"

func TestMD5Hash(t *testing.T) {
	digest := MD5Hash(key)
	assert.Equal(t, len(digest), 32)
}

func TestSHA1Hash(t *testing.T) {
	digest := SHA1Hash(key)
	assert.Equal(t, len(digest), 40)
}

func TestSHA256Hash(t *testing.T) {
	digest := SHA256Hash(key)
	assert.Equal(t, len(digest), 64)
}
