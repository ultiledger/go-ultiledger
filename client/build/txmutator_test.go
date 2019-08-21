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
	"strings"
	"testing"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

func TestAccountMutator(t *testing.T) {
	// Create a random account.
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	// Test AccountID mutator with a valid account key.
	accID := AccountID{AccountID: pk}
	err = accID.Mutate(tx)
	assert.Nil(t, err)
	assert.Equal(t, tx.AccountID, accID.AccountID)

	// Test AccountID mutator with a invaid account key.
	accID = AccountID{AccountID: "InvalidID"}
	err = accID.Mutate(tx)
	assert.NotNil(t, err)
}

func TestNoteMutator(t *testing.T) {
	// Create a long note.
	var strs []string
	for i := 0; i < 1024; i++ {
		strs = append(strs, "X")
	}
	note := strings.Join(strs, "")

	tx := &ultpb.Tx{}

	// Test Note mutator with a invalid note.
	n := Note{Note: note}
	err := n.Mutate(tx)
	assert.NotNil(t, err)
}

func TestCreateAccountMutator(t *testing.T) {
	// Create a random account.
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	ca := CreateAccount{
		AccountID: pk,
		Balance:   int64(1000),
	}
	err = ca.Mutate(tx)
	assert.Nil(t, err)

	ca.Balance = BaseFee - 1
	err = ca.Mutate(tx)
	assert.NotNil(t, err)
}

func TestPaymentMutator(t *testing.T) {
	// Create random account.
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	p := Payment{
		AccountID: pk,
		Amount:    int64(1000),
		Asset:     &Asset{AssetName: "ABC", Issuer: pk, AssetType: NATIVE},
	}
	err = p.Mutate(tx)
	assert.Nil(t, err)

	// Test an invalid payment amount.
	p.Amount = int64(-1)
	err = p.Mutate(tx)
	assert.NotNil(t, err)
	p.Amount = int64(1000)

	// Test an invalid asset name.
	p.Asset.AssetName = "ABCDE"
	err = p.Mutate(tx)
	assert.NotNil(t, err)

	// Test an invalid asset type.
	p.Asset.AssetName = "ABC"
	p.Asset.AssetType = AssetType(2)
	err = p.Mutate(tx)
	assert.NotNil(t, err)
}

func TestTrustMutator(t *testing.T) {
	// Create a random account.
	pk, _, err := crypto.GetAccountKeypair()
	assert.Nil(t, err)

	tx := &ultpb.Tx{}

	trust := Trust{
		Asset: &Asset{AssetName: "ABC", Issuer: pk, AssetType: CUSTOM},
		Limit: 100,
	}
	err = trust.Mutate(tx)
	assert.Nil(t, err)

	// Test an invalid trust limit.
	trust.Limit = int64(-1)
	err = trust.Mutate(tx)
	assert.NotNil(t, err)
}
