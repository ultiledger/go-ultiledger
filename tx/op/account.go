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
	Balance      uint64
}

func (op *CreateAccount) Apply(dt db.Tx) error {
	// validate parameters
	if op.SrcAccountID == op.DstAccountID {
		return errors.New("src and dst account is the same")
	}
	if op.Balance == 0 {
		return errors.New("init balance for dst account is zero")
	}

	// get src account
	srcAcc, err := op.AM.GetAccount(dt, op.SrcAccountID)
	if err != nil {
		return fmt.Errorf("get src account %s failed: %v", op.SrcAccountID, err)
	}

	// check src account has enough ULUs
	if srcAcc.Balance < op.Balance {
		return fmt.Errorf("src account is underfund")
	}

	// update the src account
	srcAcc.Balance -= op.Balance
	err = op.AM.SaveAccount(dt, srcAcc)
	if err != nil {
		return fmt.Errorf("update account %s failed: %v", op.SrcAccountID, err)
	}

	// create the dst account
	err = op.AM.CreateAccount(dt, op.DstAccountID, op.Balance, op.SrcAccountID)
	if err != nil {
		return fmt.Errorf("create account %s failed: %v", op.DstAccountID, err)
	}

	return nil
}
