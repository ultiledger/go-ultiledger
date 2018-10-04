package op

import (
	"github.com/ultiledger/go-ultiledger/account"
)

// Operation for creating a new account
type CreateAccount struct {
	am           *account.Manager
	srcAccountID string
	dstAccountID string
	balance      uint64
}

func (op *CreateAccount) Apply() error {
	return nil
}
