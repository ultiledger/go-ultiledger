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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testSeed       string = "rxQoxmefYtC3RmyZU11XgcYezNZsUA2Ayik5oeTWdfga"
	testPublicKey  string = "PSJdZ8VqyEm1cX6fjiVBDsjrMqgn3NemV6bYDPnNuZQS"
	testPrivateKey string = "e63e868ca423ed97006664b95e3d7f61f10fd61326b59c0c06d6e1c3e15e573f4d5c9132eade89cbba84cb6373198c51373b0ecbc09d069c84504f2bcfd0c7df"
	testSignature  string = "25YcdK5GdEXLCbg3eB6R7HBhNAufZt6D8JABpW7j2tamxjcABuVema64VtsmWyCNEPWmQoBBkjsfe7RAmfsDss8K"
	testData       string = "ultiledger is awesome!"
)

// test validity of supplied key
func TestKeypair(t *testing.T) {
	_, _, err := GetAccountKeypair()
	assert.Equal(t, nil, err)
	_, _, err = GetNodeKeypair()
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
