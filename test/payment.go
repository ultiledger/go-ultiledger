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

package test

import (
	"errors"
	"fmt"
	"time"

	"github.com/ultiledger/go-ultiledger/client"
	"github.com/ultiledger/go-ultiledger/client/build"
	"github.com/ultiledger/go-ultiledger/client/types"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
)

func init() {
	Register(&OneToOnePayment{})
}

var baseFee = ledger.GenesisBaseFee

// OneToOnePayment tests the correctness of a point-to-point payment.
type OneToOnePayment struct{}

func (p *OneToOnePayment) Desc() string {
	return "testcase: one-to-one payment"
}

func (cta *OneToOnePayment) Run(c *client.GrpcClient) error {
	// Generate a key pair for the destination account.
	srcAccountID, srcSeed, err := crypto.GetAccountKeypair()
	if err != nil {
		return fmt.Errorf("get source account keypair failed: %v", err)
	}
	// Create the first test account.
	srcAccount, err := c.CreateTestAccount(srcAccountID)
	if err != nil {
		return fmt.Errorf("create test source account failed: %v", err)
	}
	if srcAccount.AccountID != srcAccountID {
		return errors.New("test source account with mismatch account id")
	}
	if srcAccount.Balance != int64(100000000000) {
		return fmt.Errorf("test source account with unexpected balance: %d", srcAccount.Balance)
	}

	// Generate a key pair for the destination account.
	dstAccountID, _, err := crypto.GetAccountKeypair()
	if err != nil {
		return fmt.Errorf("get destination account keypair failed: %v", err)
	}
	// Create the second test account.
	dstAccount, err := c.CreateTestAccount(dstAccountID)
	if err != nil {
		return fmt.Errorf("create test destination account failed: %v", err)
	}
	if dstAccount.AccountID != dstAccountID {
		return errors.New("test destination account with mismatch account id")
	}
	if dstAccount.Balance != int64(100000000000) {
		return fmt.Errorf("test account with unexpected balance: %d", dstAccount.Balance)
	}

	// Mutators are operators to maniputate the transaction.
	var mutators []build.TxMutator
	mutators = append(mutators, &build.AccountID{AccountID: srcAccountID})
	mutators = append(mutators, &build.Payment{
		AccountID: dstAccountID,
		Amount:    int64(10000000000), // Pay 1 ULT
		Asset:     &build.Asset{AssetType: build.NATIVE},
	})
	mutators = append(mutators, &build.SeqNum{SeqNum: srcAccount.SeqNum + 1})

	// Apply the mutators to the transaction.
	tx := build.NewTx()
	err = tx.Add(mutators...)
	if err != nil {
		return fmt.Errorf("build tx failed: %v", err)
	}

	payload, signature, err := tx.Sign(srcSeed)
	if err != nil {
		return fmt.Errorf("sign payment tx failed: %v", err)
	}
	txKey, err := tx.GetTxKey()
	if err != nil {
		return fmt.Errorf("get tx fee failed: %v", err)
	}

	// Submit the tx.
	err = c.SubmitTx(txKey, signature, payload)
	if err != nil {
		return fmt.Errorf("submit tx failed: %v", err)
	}

	// Wait until the tx be confirmed.
	ticker := time.NewTicker(2 * time.Second)
	timer := time.NewTimer(30 * time.Second)
	for {
		select {
		case <-ticker.C:
			// Check the tx.
			status, err := c.QueryTx(txKey)
			if err != nil {
				return fmt.Errorf("query tx failed: %v", err)
			}
			switch status.StatusCode {
			case types.NotExist:
				return errors.New("tx not found")
			case types.Rejected:
				return fmt.Errorf("tx rejected: %v", status.ErrorMessage)
			case types.Accepted:
				continue
			case types.Confirmed:
				log.Infow("the tx is confirmed", "txKey", txKey)
				srcAcc, err := c.GetAccount(srcAccount.AccountID)
				if err != nil {
					return fmt.Errorf("get account failed: %v", err)
				}
				dstAcc, err := c.GetAccount(dstAccount.AccountID)
				if err != nil {
					return fmt.Errorf("get account failed: %v", err)
				}
				// Check the balance of the accounts.
				if srcAcc.Balance != int64(100000000000)-int64(10000000000)-baseFee {
					return fmt.Errorf("src account with unexpected balance: %d", srcAcc.Balance)
				}
				if dstAcc.Balance != int64(100000000000)+int64(10000000000) {
					return fmt.Errorf("dst account with unexpected balance: %d", dstAcc.Balance)
				}
				return nil
			case types.Failed:
				return fmt.Errorf("tx failed: %v", status.ErrorMessage)
			default:
				return errors.New("tx status unknown")
			}
		case <-timer.C:
			return errors.New("query result takes too long")
		}
	}

	return nil
}
