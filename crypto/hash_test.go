package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var key string = "hello world!"

func TestMD5Hash(t *testing.T) {
	digest := MD5Hash([]byte(key))
	assert.Equal(t, len(digest), 22)
}
func TestSHA1Hash(t *testing.T) {
	digest := SHA1Hash([]byte(key))
	assert.Equal(t, len(digest), 27)
}
func TestSHA256Hash(t *testing.T) {
	digest := SHA256Hash([]byte(key))
	assert.Equal(t, len(digest), 44)
}
