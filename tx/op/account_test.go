package op

import (
	"testing"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db/memdb"

	"github.com/stretchr/testify/assert"
)

const (
	signer     = "SIGNER"
	srcAccount = "srcAccount"
	dstAccount = "dstAccount"
)

func TestAccountOp(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)

	// create source account
	err := am.CreateAccount(memorydb, srcAccount, 1000000, signer, 2)
	assert.Equal(t, err, nil)

	memorytx, _ := memorydb.Begin()

	// create account op
	accountOp := CreateAccount{
		AM:           am,
		SrcAccountID: srcAccount,
		DstAccountID: dstAccount,
		Balance:      100000,
		BaseReserve:  100,
		SeqNum:       3,
	}
	err = accountOp.Apply(memorytx)
	assert.Equal(t, err, nil)

	// check dst account
	dstAcc, err := am.GetAccount(memorytx, dstAccount)
	assert.Equal(t, err, nil)
	assert.Equal(t, dstAcc.Balance, int64(100000))

	// check src account
	srcAcc, err := am.GetAccount(memorytx, srcAccount)
	assert.Equal(t, err, nil)
	assert.Equal(t, srcAcc.Balance, int64(900000))
}
