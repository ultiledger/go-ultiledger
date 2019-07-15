package crypto

import (
	"bytes"
	"encoding/binary"
	"errors"

	b58 "github.com/mr-tron/base58/base58"
)

type KeyType uint8

// Enumeration of key type.
const (
	_ KeyType = iota // skip zero
	KeyTypeAccountID
	KeyTypeSeed
	KeyTypeTx
	KeyTypeTxSet
	KeyTypeNodeID
	KeyTypeLedgerHeader
	KeyTypeOfferID
)

var (
	ErrInvalidKey = errors.New("invalid key string")
)

// ULTKey is the internal key to represent various key hash,
// Code is for identifying the type of certain key hash.
type ULTKey struct {
	Code KeyType
	Hash [32]byte
}

// Decode base58 encoded key string to ULTKey.
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
	case KeyTypeAccountID:
		fallthrough
	case KeyTypeSeed:
		fallthrough
	case KeyTypeTx:
		fallthrough
	case KeyTypeTxSet:
		fallthrough
	case KeyTypeLedgerHeader:
		fallthrough
	case KeyTypeNodeID:
		return &ultKey, nil
	}
	return nil, ErrInvalidKey
}

// Encode UTLKey to base58 encoded key string.
func EncodeKey(ultKey *ULTKey) string {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, ultKey)
	return b58.Encode(buf.Bytes())
}

// Check the validity of supplied key string.
func IsValidKey(key string) bool {
	if _, err := DecodeKey(key); err != nil {
		return false
	}
	return true
}

// Check the validity of supplied acount key string.
func IsValidAccountKey(key string) bool {
	ultKey, err := DecodeKey(key)
	if err != nil {
		return false
	}
	if ultKey.Code != KeyTypeAccountID {
		return false
	}
	return true
}

// Check the validity of supplied tx key string.
func IsValidTxKey(key string) bool {
	ultKey, err := DecodeKey(key)
	if err != nil {
		return false
	}
	if ultKey.Code != KeyTypeTx {
		return false
	}
	return true
}
