package account

import (
	"errors"
	"fmt"

	pb "github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrAccountNotExist = errors.New("account not exist")
)

// Manager manages the creation of accounts
type Manager struct {
	database db.Database
	bucket   string

	// LRU cache for accounts
	accounts *lru.Cache

	// master account
	master *ultpb.Account
}

func NewManager(d db.Database) *Manager {
	am := &Manager{
		database: d,
		bucket:   "ACCOUNT",
	}
	err := am.database.NewBucket(am.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", am.bucket, err)
	}
	cache, err := lru.New(10000)
	if err != nil {
		log.Fatalf("create account manager LRU cache failed: %v", err)
	}
	am.accounts = cache
	return am
}

// Create master account with native asset (ULT) and initial balances
func (am *Manager) CreateMasterAccount(networkID []byte, balance uint64) error {
	pubKey, privKey, err := crypto.GetAccountKeypairFromSeed(networkID)
	if err != nil {
		return err
	}
	log.Infof("master private key (seed) is %s", privKey)

	err = am.CreateAccount(pubKey, balance, pubKey)
	if err != nil {
		return fmt.Errorf("create master account failed: %v", err)
	}

	return nil
}

// Create a new account with initial balance
func (am *Manager) CreateAccount(accountID string, balance uint64, signer string) error {
	acc := &ultpb.Account{
		AccountID: accountID,
		Balance:   balance,
		Signer:    signer,
	}

	accb, err := ultpb.Encode(acc)
	if err != nil {
		return fmt.Errorf("encode account failed: %v", err)
	}

	// save the account in db
	err = am.database.Put(am.bucket, []byte(acc.AccountID), accb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	// save the account in cache
	am.accounts.Add(acc.AccountID, acc)

	return nil
}

// Get account information from accountID
func (am *Manager) GetAccount(accountID string) (*ultpb.Account, error) {
	// first check the LRU cache, if the account is in the cache
	// we return a deep copy of the account
	if acc, ok := am.accounts.Get(accountID); ok {
		a := acc.(*ultpb.Account)
		accCopy := pb.Clone(a)
		return accCopy.(*ultpb.Account), nil
	}

	// then check database
	b, err := am.database.Get(am.bucket, []byte(accountID))
	if err != nil {
		return nil, fmt.Errorf("get account %s failed: %v", accountID, err)
	}
	if b == nil {
		return nil, ErrAccountNotExist
	}
	acc, err := ultpb.DecodeAccount(b)
	if err != nil {
		return nil, fmt.Errorf("account %s decode failed: %v", accountID, err)
	}

	// cache the account
	am.accounts.Add(accountID, acc)
	accCopy := pb.Clone(acc)

	return accCopy.(*ultpb.Account), nil
}

// Update account information
func (am *Manager) UpdateAccount(acc *ultpb.Account) error {
	oldAcc, err := am.GetAccount(acc.AccountID)
	if err != nil {
		if err == ErrAccountNotExist {
			return err
		}
		return fmt.Errorf("get account failed: %v", err)
	}

	// we cannot change the signer of the account
	if oldAcc.Signer != acc.Signer {
		return fmt.Errorf("cannot update account signer")
	}

	accb, err := ultpb.Encode(acc)
	if err != nil {
		return fmt.Errorf("encode account failed: %v", err)
	}

	// update account in db
	err = am.database.Put(am.bucket, []byte(acc.AccountID), accb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	am.accounts.Add(acc.AccountID, acc)

	return nil
}
