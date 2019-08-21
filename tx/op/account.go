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
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
)

var (
	ErrAccountNotExist = errors.New("account not exist")
)

// Operation for creating a new account.
type CreateAccount struct {
	AM           *account.Manager
	SrcAccountID string
	DstAccountID string
	Balance      int64
	BaseReserve  int64
	LedgerSeqNum uint64
}

func (c *CreateAccount) Apply(dt db.Tx) error {
	// Sanity checks.
	if c.SrcAccountID == c.DstAccountID {
		return errors.New("src and dst account is the same")
	}
	if c.Balance == 0 {
		return errors.New("init balance for dst account is zero")
	}

	// Check whether the destination account exists.
	dstAcc, err := c.AM.GetAccount(dt, c.DstAccountID)
	if err != nil {
		return fmt.Errorf("get dst account %s failed: %v", c.DstAccountID, err)
	}
	if dstAcc != nil {
		return errors.New("dst account already exists")
	}

	// Get source account.
	srcAcc, err := c.AM.GetAccount(dt, c.SrcAccountID)
	if err != nil {
		return fmt.Errorf("get src account %s failed: %v", c.SrcAccountID, err)
	}
	if srcAcc == nil {
		return ErrAccountNotExist
	}

	// Check source account has enough ULU.
	if srcAcc.Balance < c.Balance {
		return errors.New("src account is underfund")
	}

	if c.Balance < c.BaseReserve {
		return errors.New("init balance is smaller than base reserve")
	}

	// Update the source account.
	srcAcc.Balance -= c.Balance
	err = c.AM.SaveAccount(dt, srcAcc)
	if err != nil {
		return fmt.Errorf("update account %s failed: %v", c.SrcAccountID, err)
	}

	// Create the destination account.
	err = c.AM.CreateAccount(dt, c.DstAccountID, c.Balance, c.SrcAccountID, c.LedgerSeqNum)
	if err != nil {
		return fmt.Errorf("create account %s failed: %v", c.DstAccountID, err)
	}

	return nil
}
