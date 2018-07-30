package crypto

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
)

// compute md5 checksum (16 bytes)
func MD5Hash(key string) string {
	v := md5.Sum([]byte(key))
	return fmt.Sprintf("%x", v)
}

// compute sha1 checksum (20 bytes)
func SHA1Hash(key string) string {
	v := sha1.Sum([]byte(key))
	return fmt.Sprintf("%x", v)
}

// compute sha256 checksum (32 bytes)
func SHA256Hash(key string) string {
	v := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", v)
}
