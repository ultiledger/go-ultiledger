package crypto

import (
	"bytes"
	"encoding/binary"
	"errors"

	b58 "github.com/mr-tron/base58/base58"
)

type KeyType uint8

// enumeration of key type
const (
	_ KeyType = iota // skip zero
	keyTypeAccountID
	keyTypeSecretKey
	keyTypeTransaction
	keyTypeSignature
)

var (
	ErrInvalidKey = errors.New("invalid key string")
)

// UTLKey is the internal key to represent various key hash,
// Code is for identifying the type of certain key hash.
type ULTKey struct {
	Code KeyType
	Hash [32]byte
}

// decode base58 encoded key string to ULTKey
func DecodeKey(key string) (*ULTKey, error) {
	if key == "" {
		return nil, ErrInvalidKey
	}

	b, err := b58.Decode(key)
	if err != nil {
		return nil, ErrInvalidKey
	}

	var ultKey ULTKey
	r := bytes.NewReader(b)
	err = binary.Read(r, binary.BigEndian, &ultKey)
	if err != nil {
		return nil, ErrInvalidKey
	}

	switch ultKey.Code {
	case keyTypeAccountID:
		fallthrough
	case keyTypeSecretKey:
		fallthrough
	case keyTypeTransaction:
		fallthrough
	case keyTypeSignature:
		return &ultKey, nil
	}
	return nil, ErrInvalidKey
}

// encode UTLKey to base58 encoded key string
func EncodeToKey(ultKey *ULTKey) string {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, ultKey)
	return b58.Encode(buf.Bytes())
}

// check the validity of supplied key string
func IsValidKey(key string) bool {
	if _, err := DecodeKey(key); err != nil {
		return false
	}
	return true
}
