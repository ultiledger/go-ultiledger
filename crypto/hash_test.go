package crypto

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var key string = "hello world!"

func TestMD5Hash(t *testing.T) {
	digest := MD5Hash([]byte(key))
	assert.Equal(t, len(digest), 32)
}
func TestSHA1Hash(t *testing.T) {
	digest := SHA1Hash([]byte(key))
	assert.Equal(t, len(digest), 40)
}
func TestSHA256Hash(t *testing.T) {
	digest := SHA256Hash([]byte(key))
	assert.Equal(t, len(digest), 64)
}
