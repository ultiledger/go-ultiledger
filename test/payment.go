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
	"github.com/ultiledger/go-ultiledger/ultpb"
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
	// Generate a key pair for the first account.
	pk1, seed1, err := crypto.GetAccountKeypair()
	if err != nil {
		return fmt.Errorf("get account keypair failed: %v", err)
	}
	// Create the first test account.
	account1, err := c.CreateTestAccount(pk1)
	if err != nil {
		return fmt.Errorf("create test account failed: %v", err)
	}
	if account1.AccountID != pk1 {
		return errors.New("test account with mismatch account id")
	}
	if account1.Balance != int64(100000000000) {
		return fmt.Errorf("test account with unexpected balance: %d", account1.Balance)
	}

	// Generate a key pair for the second account.
	pk2, _, err := crypto.GetAccountKeypair()
	if err != nil {
		return fmt.Errorf("get account keypair failed: %v", err)
	}
	// Create the second test account.
	account2, err := c.CreateTestAccount(pk2)
	if err != nil {
		return fmt.Errorf("create test account failed: %v", err)
	}
	if account2.AccountID != pk2 {
		return errors.New("test account with mismatch account id")
	}
	if account2.Balance != int64(100000000000) {
		return fmt.Errorf("test account with unexpected balance: %d", account2.Balance)
	}

	// Create the payment transaction.
	tx := build.NewTx()
	accID := &build.AccountID{AccountID: account1.AccountID}
	paymentMut := &build.Payment{
		AccountID: account2.AccountID,
		Amount:    int64(10000000000), // Pay 1 ULT
		Asset:     &build.Asset{AssetType: build.NATIVE},
	}
	seqNumMut := &build.SeqNum{SeqNum: account1.SeqNum + 1}

	err = tx.Add(accID, paymentMut, seqNumMut)
	if err != nil {
		return fmt.Errorf("build tx failed: %v", err)
	}
	payload, signature, err := tx.Sign(seed1)
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

	var acc1, acc2 *ultpb.Account

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
				// Get the account
				acc1, err = c.GetAccount(account1.AccountID)
				if err != nil {
					return fmt.Errorf("get account failed: %v", err)
				}
				acc2, err = c.GetAccount(account2.AccountID)
				if err != nil {
					return fmt.Errorf("get account failed: %v", err)
				}
				// Check the balance of the accounts.
				if acc1.Balance != int64(100000000000)-int64(10000000000)-baseFee {
					return fmt.Errorf("src account with unexpected balance: %d", acc1.Balance)
				}
				if acc2.Balance != int64(100000000000)+int64(10000000000) {
					return fmt.Errorf("dst account with unexpected balance: %d", acc2.Balance)
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
