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

package op

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db/memdb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

const (
	issuer = "Issuer"
)

func TestTrustOp(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)

	// Create a signer account.
	signer, _, _ := crypto.GetAccountKeypair()

	// Create a asset issuer.
	issuer, _, _ := crypto.GetAccountKeypair()
	err := am.CreateAccount(memorydb, issuer, 1000000, signer, 2)
	assert.Nil(t, err)

	// Create a source account.
	srcAccount, _, _ := crypto.GetAccountKeypair()
	err = am.CreateAccount(memorydb, srcAccount, 1000000, signer, 3)
	assert.Nil(t, err)

	// Create a destination account.
	dstAccount, _, _ := crypto.GetAccountKeypair()
	err = am.CreateAccount(memorydb, dstAccount, 1000000, signer, 4)
	assert.Nil(t, err)

	memorytx, _ := memorydb.Begin()

	// Create the trust operator.
	asset := &ultpb.Asset{
		AssetType: ultpb.AssetType_CUSTOM,
		AssetName: "COIN",
		Issuer:    issuer,
	}
	trustOp := Trust{
		AM:           am,
		SrcAccountID: srcAccount,
		Asset:        asset,
		Limit:        10000,
	}

	// Apply the trust operator.
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	// Check the entry count of source account.
	srcAcc, err := am.GetAccount(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, srcAcc.EntryCount, int32(1))

	// Check the existance of the new trust.
	trust, err := am.GetTrust(memorytx, srcAccount, asset)
	assert.Nil(t, err)
	assert.NotNil(t, trust)

	// Lower the trust limit.
	trustOp.Limit = 5000
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	// Delete the trust.
	trustOp.Limit = 0
	err = trustOp.Apply(memorytx)
	assert.Nil(t, err)

	trust, err = am.GetTrust(memorytx, srcAccount, asset)
	assert.Nil(t, err)
	assert.Nil(t, trust)

	memorytx.Commit()
}
