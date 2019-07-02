package crypto

import (
	"crypto/sha256"
	"encoding/binary"

	b58 "github.com/mr-tron/base58/base58"
)

// Compute sha256 checksum (32 bytes) and return a string.
func SHA256Hash(b []byte) string {
	v := sha256.Sum256(b)
	return b58.Encode(v[:])
}

// Compute sha256 checksum (32 bytes) and return a byte array.
func SHA256HashBytes(b []byte) [32]byte {
	return sha256.Sum256(b)
}

// Compute sha256 checksum and generate a uint64 number using
// the first 8 bytes of the byte array.
func SHA256HashUint64(b []byte) uint64 {
	bytes := sha256.Sum256(b)
	num := binary.BigEndian.Uint64(bytes[0:8])
	return num
}
