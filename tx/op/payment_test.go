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
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestPaymentOp(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)
	em := exchange.NewManager(memorydb, am)

	// Create a signer account.
	signer, _, _ := crypto.GetAccountKeypair()

	// Create a source account.
	srcAccount, _, _ := crypto.GetAccountKeypair()
	err := am.CreateAccount(memorydb, srcAccount, 1000000, signer, 2)
	assert.Nil(t, err)

	// Create a destination account.
	dstAccount, _, _ := crypto.GetAccountKeypair()
	err = am.CreateAccount(memorydb, dstAccount, 10000, signer, 3)
	assert.Nil(t, err)

	memorytx, _ := memorydb.Begin()

	// Create the payment operator.
	paymentOp := Payment{
		AM:           am,
		EM:           em,
		SrcAccountID: srcAccount,
		DstAccountID: dstAccount,
		Asset:        &ultpb.Asset{AssetType: ultpb.AssetType_NATIVE, AssetName: "ULU", Issuer: signer},
		Amount:       int64(10000),
	}
	err = paymentOp.Apply(memorytx)
	assert.Nil(t, err)

	// Check the balance of destination account.
	dstAcc, err := am.GetAccount(memorytx, dstAccount)
	assert.Nil(t, err)
	assert.Equal(t, dstAcc.Balance, int64(20000))

	// Check the balance of source account.
	srcAcc, err := am.GetAccount(memorytx, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, srcAcc.Balance, int64(990000))

	memorytx.Commit()
}
