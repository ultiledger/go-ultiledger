package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/ed25519"
)

// generate key pair with ed25510 cryto algorithm, since we can
// always reconstruct the true private key using the same seed,
// we use the randomly generated seed as a equivalent private key.
func ed25519Keypair() (string, string, error) {
	var seed [32]byte
	_, err := io.ReadFull(rand.Reader, seed[:])
	if err != nil {
		return "", "", err
	}
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public()

	pubKeyStr := fmt.Sprintf("%x", publicKey)
	seedStr := fmt.Sprintf("%x", seed)

	return pubKeyStr, seedStr, nil
}

// reconstruct the true private key from the seed, it supposes to
// be only used in situations where you need to sign the data so
// the authenticity can be verified by the corresponding public key.
func getPrivateKey(seed string) (ed25519.PrivateKey, error) {
	if seed == "" {
		return nil, fmt.Errorf("empty seed")
	}
	b, err := hex.DecodeString(seed)
	if err != nil {
		return nil, err
	}
	privateKey := ed25519.NewKeyFromSeed(b)
	return privateKey, nil
}

// randomly generate a pair of public and private key
func GenerateKeypair() (string, string, error) {
	// privateKey is actually the seed used to generate the keypair
	publicKey, seed, err := ed25519Keypair()
	if err != nil {
		return "", "", err
	}
	return publicKey, seed, err
}

func GenerateKeypairFromSeed(seed []byte) (string, string, error) {
	if len(seed) != 32 {
		return "", "", errors.New("Invalid seed, byte length is not 32")
	}
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public()

	pubKeyStr := fmt.Sprintf("%x", publicKey)
	seedStr := fmt.Sprintf("%x", seed)

	return pubKeyStr, seedStr, nil

}

// sign the data with provided seed (equivalent private key)
func Sign(seed string, data []byte) (string, error) {
	pk, err := getPrivateKey(seed)
	if err != nil {
		return "", err
	}
	signature := ed25519.Sign(pk, data)
	return hex.EncodeToString(signature), nil
}

// verify the data signature
func Verify(publicKey, signature string, data []byte) bool {
	b, err := hex.DecodeString(publicKey)
	if err != nil {
		return false
	}
	s, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	pub := ed25519.PublicKey(b)
	return ed25519.Verify(pub, data, s)
}
