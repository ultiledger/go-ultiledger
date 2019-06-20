package op

import (
	"errors"
	"fmt"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
)

// Operation for creating a new account.
type CreateAccount struct {
	AM           *account.Manager
	SrcAccountID string
	DstAccountID string
	Balance      int64
}

func (c *CreateAccount) Apply(dt db.Tx) error {
	// validate parameters
	if c.SrcAccountID == c.DstAccountID {
		return errors.New("src and dst account is the same")
	}
	if c.Balance == 0 {
		return errors.New("init balance for dst account is zero")
	}

	// get src account
	srcAcc, err := c.AM.GetAccount(dt, c.SrcAccountID)
	if err != nil {
		return fmt.Errorf("get src account %s failed: %v", c.SrcAccountID, err)
	}

	// check src account has enough ULUs
	if srcAcc.Balance < c.Balance {
		return fmt.Errorf("src account is underfund")
	}

	// update the src account
	srcAcc.Balance -= c.Balance
	err = c.AM.SaveAccount(dt, srcAcc)
	if err != nil {
		return fmt.Errorf("update account %s failed: %v", c.SrcAccountID, err)
	}

	// create the dst account
	err = c.AM.CreateAccount(dt, c.DstAccountID, c.Balance, c.SrcAccountID)
	if err != nil {
		return fmt.Errorf("create account %s failed: %v", c.DstAccountID, err)
	}

	return nil
}
