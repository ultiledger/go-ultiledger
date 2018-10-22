package account

import (
	"errors"
	"fmt"
	"math"

	pb "github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrAccountNotExist  = errors.New("account not exist")
	ErrBalanceOverflow  = errors.New("account balance overflow")
	ErrBalanceUnderflow = errors.New("account balance underflow")
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

	err = am.CreateAccount(am.database, pubKey, balance, pubKey)
	if err != nil {
		return fmt.Errorf("create master account failed: %v", err)
	}

	return nil
}

// Create a new account with initial balance
func (am *Manager) CreateAccount(putter db.Putter, accountID string, balance uint64, signer string) error {
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
	err = putter.Put(am.bucket, []byte(acc.AccountID), accb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	// save the account in cache
	am.accounts.Add(acc.AccountID, acc)

	return nil
}

// Get account information from accountID
func (am *Manager) GetAccount(getter db.Getter, accountID string) (*ultpb.Account, error) {
	// first check the LRU cache, if the account is in the cache
	// we return a deep copy of the account
	if acc, ok := am.accounts.Get(accountID); ok {
		a := acc.(*ultpb.Account)
		accCopy := pb.Clone(a)
		return accCopy.(*ultpb.Account), nil
	}

	// then check database
	b, err := getter.Get(am.bucket, []byte(accountID))
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

// Update account information.
func (am *Manager) SaveAccount(putter db.Putter, acc *ultpb.Account) error {
	accb, err := ultpb.Encode(acc)
	if err != nil {
		return fmt.Errorf("encode account failed: %v", err)
	}

	// update account in db
	err = putter.Put(am.bucket, []byte(acc.AccountID), accb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	return nil
}

// Add balance to account and check balance overflow.
func (am *Manager) AddBalance(acc *ultpb.Account, balance uint64) error {
	if acc.Balance > math.MaxUint64-balance {
		return ErrBalanceOverflow
	}

	acc.Balance += balance

	return nil
}

// Subtract balance from account and check balance underflow
func (am *Manager) SubBalance(acc *ultpb.Account, balance uint64) error {
	if acc.Balance < balance {
		return ErrBalanceUnderflow
	}

	acc.Balance -= balance

	return nil
}
