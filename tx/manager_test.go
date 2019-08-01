package tx

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db/memdb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestApplyTxList(t *testing.T) {
	memorydb := memdb.New()
	am := account.NewManager(memorydb, 100)
	tm := Manager{database: memorydb, am: am}

	// Create a signer account.
	signer, _, _ := crypto.GetAccountKeypair()

	// Create a source account.
	srcAccount, _, _ := crypto.GetAccountKeypair()
	err := am.CreateAccount(memorydb, srcAccount, 1000000, signer, 2)
	assert.Nil(t, err)

	// Create three destination account.
	var accounts []string
	for i := 0; i < 3; i++ {
		dstAccount, _, _ := crypto.GetAccountKeypair()
		err = am.CreateAccount(memorydb, dstAccount, 10000, signer, uint64(i+3))
		assert.Nil(t, err)
		accounts = append(accounts, dstAccount)
	}

	// Create three payment operations to transfer asset from the source
	// account to the three destination accounts.
	var ops []*ultpb.Op
	for i := 0; i < 3; i++ {
		paymentOp := &ultpb.PaymentOp{
			AccountID: accounts[i],
			Asset:     &ultpb.Asset{AssetType: ultpb.AssetType_NATIVE, AssetName: "ULU", Issuer: signer},
			Amount:    int64(10000),
		}
		ops = append(ops, &ultpb.Op{
			OpType: ultpb.OpType_PAYMENT,
			Op:     &ultpb.Op_Payment{Payment: paymentOp},
		})
	}

	// Create a transaction.
	tx := &ultpb.Tx{
		AccountID: srcAccount,
		Fee:       int64(100),
		SeqNum:    uint64(6),
		OpList:    ops,
	}
	err = tm.ApplyTxList([]*ultpb.Tx{tx}, 6)
	assert.Nil(t, err)

	// Check the balances of the accounts.
	srcAcc, err := am.GetAccount(memorydb, srcAccount)
	assert.Nil(t, err)
	assert.Equal(t, int64(969900), srcAcc.Balance)

	for i := 0; i < 3; i++ {
		acc, err := am.GetAccount(memorydb, accounts[i])
		assert.Nil(t, err)
		assert.Equal(t, int64(20000), acc.Balance)
	}
}
