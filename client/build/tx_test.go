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

package build

import (
	"testing"

	"github.com/ultiledger/go-ultiledger/crypto"

	"github.com/stretchr/testify/assert"
)

func TestTx(t *testing.T) {
	// Create a random account.
	src, seed, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	dst, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := NewTx()

	// AccountID mutator.
	accID := &AccountID{AccountID: src}

	// Note mutator.
	n := &Note{Note: "SIMPLE NOTE"}

	// CreateAcount mutator.
	ca := &CreateAccount{
		AccountID: dst,
		Balance:   int64(1000),
	}

	// Payment mutator.
	p := &Payment{
		AccountID: dst,
		Amount:    int64(1000),
		Asset:     &Asset{AssetName: "ABC", Issuer: src, AssetType: NATIVE},
	}

	// Trust mutator.
	trust := &Trust{
		Asset: &Asset{AssetName: "XYZ", Issuer: dst, AssetType: CUSTOM},
		Limit: 100,
	}

	err = tx.Add(accID, n, ca, p, trust)
	assert.Nil(t, err)

	assert.Equal(t, tx.Tx.Fee, int64(3)*BaseFee)

	// Testing signing the tx.
	payload, signature, err := tx.Sign(seed)
	assert.Nil(t, err)

	result := crypto.Verify(src, signature, payload)
	assert.True(t, result)
}
