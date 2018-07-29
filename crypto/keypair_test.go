package crypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testSeed       string = "3ac371907afa714b434af49b7a215fd1010d8561c22ef987a47e078549ef21a9"
	testPublicKey  string = "9f6a604c5091f713584f2b9e48d0497106bc70335914f0d19c6df3b7b038e01b"
	testPrivateKey string = "3ac371907afa714b434af49b7a215fd1010d8561c22ef987a47e078549ef21a99f6a604c5091f713584f2b9e48d0497106bc70335914f0d19c6df3b7b038e01b"
	testSignature  string = "930a1a48958f2393e75a6844d4555a43244cedb321d86ad35c9342db8818fd1330e40f63555ed8c0e5cb1fb2a61cbb91ffb2fb9dd80f2229ef96373520d2940d"
	testData       string = "ultiledger is awesome!"
)

// test validity of supplied key
func TestKeypair(t *testing.T) {
	_, _, err := GenerateKeypair()
	assert.Equal(t, nil, err)
}

// test get privagte key from seed
func TestPrivateKey(t *testing.T) {
	pk, err := getPrivateKey(testSeed)
	assert.Equal(t, nil, err)
	assert.Equal(t, testPrivateKey, fmt.Sprintf("%x", pk))
}

// test data signing and verification
func TestSignAndVerify(t *testing.T) {
	signature, err := Sign(testSeed, []byte(testData))
	assert.Equal(t, nil, err)
	assert.Equal(t, testSignature, signature)
	assert.Equal(t, true, Verify(testPublicKey, signature, []byte(testData)))
}
