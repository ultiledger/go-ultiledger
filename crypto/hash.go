package crypto

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"

	"github.com/golang/protobuf/proto"

	"github.com/ultiledger/go-ultiledger/ultpb"
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

// compute sha256 checksum of proto message
func SHA256HashPb(msg proto.Message) (string, error) {
	b, err := ultpb.Encode(msg)
	if err != nil {
		return "", err
	}
	return SHA256Hash(b), nil
}
