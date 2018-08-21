package crypto

import (
	"bytes"
	"encoding/binary"
	"testing"

	b58 "github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
)

var (
	testHash           string = "05319d6e01057b489715b5c0cf9562059595a6d2cbbd0a080360937b82f831fc" // 32 bytes ed25519 key
	testKeyAccountID   string = "MUVzSrCzNYTfGEZEwYSkn5zykhhd1MJNaXtezC8PuBat"
	testKeySecretKey   string = "ehpMwdxbAbCRqKLfAid3yJoW1CLwkZNU4aqyr7ZAR8qA"
	testKeyTransaction string = "ww8jSRiBxdwCQQ85PtoMAXc2FgzGVmSZYdoJi2yvw65S"
	testKeySignature   string = "25YcdK5GdEXLCbg3eB6R7HBhNAufZt6D8JABpW7j2tamxjcABuVema64VtsmWyCNEPWmQoBBkjsfe7RAmfsDss8K"
)

// test validity of supplied key
func TestKeyValidity(t *testing.T) {
	assert.Equal(t, true, IsValidKey(testKeyAccountID))
	assert.Equal(t, true, IsValidKey(testKeySecretKey))
	assert.Equal(t, true, IsValidKey(testKeyTransaction))

	// test empty key string
	assert.Equal(t, false, IsValidKey(""))

	// construct an invalid key type
	tk := ULTKey{Code: KeyType(128)}
	copy(tk.Hash[:], testHash)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, tk)

	b58code := b58.Encode(buf.Bytes())
	assert.Equal(t, false, IsValidKey(b58code))
}

// test base58 encoding of AccountID key
func TestKeyAccountID(t *testing.T) {
	tk := ULTKey{Code: KeyTypeAccountID}
	copy(tk.Hash[:], testHash)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, tk)

	b58code := b58.Encode(buf.Bytes())
	assert.Equal(t, testKeyAccountID, b58code)
}

// test base58 encoding of Seed key
func TestKeySeed(t *testing.T) {
	tk := ULTKey{Code: KeyTypeSeed}
	copy(tk.Hash[:], testHash)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, tk)

	b58code := b58.Encode(buf.Bytes())
	assert.Equal(t, testKeySecretKey, b58code)
}

// test base58 encoding of Transaction key
func TestKeyTransaction(t *testing.T) {
	tk := ULTKey{Code: KeyTypeTransaction}
	copy(tk.Hash[:], testHash)

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, tk)

	b58code := b58.Encode(buf.Bytes())
	assert.Equal(t, testKeyTransaction, b58code)
}
