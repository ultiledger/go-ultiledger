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

package testcase

import (
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/client"
	"github.com/ultiledger/go-ultiledger/crypto"
)

func init() {
	Register(&CreateTestAccount{})
}

// CreateTestAccount tests the correctness of creating a test account.
type CreateTestAccount struct{}

func (cta *CreateTestAccount) Run(c *client.GrpcClient) error {
	// Generate a key pair.
	pk, _, err := crypto.GetAccountKeypair()
	if err != nil {
		return fmt.Errorf("get account keypair failed: %v", err)
	}
	// Create the test account.
	account, err := c.CreateTestAccount(pk)
	if err != nil {
		return fmt.Errorf("create test account failed: %v", err)
	}
	if account.AccountID != pk {
		return errors.New("test account with mismatch account id")
	}
	if account.Balance != int64(100000000000) {
		return fmt.Errorf("test account with unexpected balance: %d", account.Balance)
	}
	return nil
}
