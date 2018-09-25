package crypto

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
)

// compute md5 checksum (16 bytes)
func MD5Hash(b []byte) string {
	v := md5.Sum(b)
	return fmt.Sprintf("%x", v)
}

// compute sha1 checksum (20 bytes)
func SHA1Hash(b []byte) string {
	v := sha1.Sum(b)
	return fmt.Sprintf("%x", v)
}

// compute sha256 checksum (32 bytes)
func SHA256Hash(b []byte) string {
	v := sha256.Sum256(b)
	return fmt.Sprintf("%x", v)
}

// compute sha256 checksum (32 bytes)
func SHA256HashBytes(b []byte) [32]byte {
	return sha256.Sum256(b)
}
