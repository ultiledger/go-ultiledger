package crypto

import (
	"crypto/rand"
	"io"

	"golang.org/x/crypto/ed25519"
)

// Generate random offerID
func GetOfferID() (string, error) {
	var seed [32]byte
	_, err := io.ReadFull(rand.Reader, seed[:])
	if err != nil {
		return "", err
	}
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	var pk [32]byte
	copy(pk[:], publicKey)

	offerID := &ULTKey{Code: KeyTypeOfferID, Hash: pk}

	offerIDStr := EncodeKey(offerID)

	return offerIDStr, nil
}
