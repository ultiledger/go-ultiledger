// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crypto

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	b58 "github.com/mr-tron/base58/base58"
	"golang.org/x/crypto/ed25519"
)

// Generate account keypair with ed25510 cryto algorithm, since we can
// always reconstruct the true private key using the same seed, we use
// the randomly generated seed as a equivalent private key.
func accountKeypair() (string, string, error) {
	var seed [32]byte
	_, err := io.ReadFull(rand.Reader, seed[:])
	if err != nil {
		return "", "", err
	}
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	var pk [32]byte
	copy(pk[:], publicKey)
	acc := &ULTKey{Code: KeyTypeAccountID, Hash: pk}
	sd := &ULTKey{Code: KeyTypeSeed, Hash: seed}

	pubKeyStr := EncodeKey(acc)
	seedStr := EncodeKey(sd)

	return pubKeyStr, seedStr, nil
}

// Generate node keypair with ed25510 cryto algorithm, since we can
// always reconstruct the true private key using the same seed, we use
// the randomly generated seed as a equivalent private key.
func nodeKeypair() (string, string, error) {
	var seed [32]byte
	_, err := io.ReadFull(rand.Reader, seed[:])
	if err != nil {
		return "", "", err
	}
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)

	var pk [32]byte
	copy(pk[:], publicKey)
	acc := &ULTKey{Code: KeyTypeNodeID, Hash: pk}
	sd := &ULTKey{Code: KeyTypeSeed, Hash: seed}

	pubKeyStr := EncodeKey(acc)
	seedStr := EncodeKey(sd)

	return pubKeyStr, seedStr, nil
}

// Reconstruct the true private key from the seed, it supposes to
// be only used in situations where you need to sign the data so
// the authenticity can be verified by the corresponding public key.
func getPrivateKey(seed string) (ed25519.PrivateKey, error) {
	if seed == "" {
		return nil, fmt.Errorf("empty seed")
	}
	k, err := DecodeKey(seed)
	if err != nil {
		return nil, err
	}
	privateKey := ed25519.NewKeyFromSeed(k.Hash[:])
	return privateKey, nil
}

// Randomly generate a pair of account public and private key.
func GetAccountKeypair() (string, string, error) {
	// privateKey is actually the seed used to generate the keypair
	publicKey, seed, err := accountKeypair()
	if err != nil {
		return "", "", err
	}
	return publicKey, seed, err
}

// Randomly generate a pair of node public and private key.
func GetNodeKeypair() (string, string, error) {
	// privateKey is actually the seed used to generate the keypair
	publicKey, seed, err := nodeKeypair()
	if err != nil {
		return "", "", err
	}
	return publicKey, seed, err
}

// Generate account keypair from provided seed.
func GetAccountKeypairFromSeed(seed []byte) (string, string, error) {
	if len(seed) != 32 {
		return "", "", errors.New("Invalid seed, byte length is not 32")
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	var pk [32]byte
	copy(pk[:], publicKey)
	acc := &ULTKey{Code: KeyTypeAccountID, Hash: pk}

	var sdk [32]byte
	copy(sdk[:], seed)
	sd := &ULTKey{Code: KeyTypeSeed, Hash: sdk}

	pubKeyStr := EncodeKey(acc)
	seedStr := EncodeKey(sd)

	return pubKeyStr, seedStr, nil

}

// Sign the data with provided seed (equivalent private key).
func Sign(seed string, data []byte) (string, error) {
	pk, err := getPrivateKey(seed)
	if err != nil {
		return "", err
	}

	signature := ed25519.Sign(pk, data)
	signStr := b58.Encode(signature)

	return signStr, nil
}

// Verify the data signature with encoded string representation
// of the public key.
func Verify(publicKey, signature string, data []byte) bool {
	pk, err := DecodeKey(publicKey)
	if err != nil {
		return false
	}
	return VerifyByKey(pk, signature, data)
}

// Verify the data signature using ULTKey.
func VerifyByKey(pk *ULTKey, signature string, data []byte) bool {
	sn, err := b58.Decode(signature)
	if err != nil {
		return false
	}
	pub := ed25519.PublicKey(pk.Hash[:])
	return ed25519.Verify(pub, data, sn)
}
