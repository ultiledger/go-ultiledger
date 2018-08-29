package account

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Manager manages the creation of accounts
type Manager struct {
	store  db.DB
	bucket string

	// LRU cache for accounts
	accounts *lru.Cache

	// master account
	master *ultpb.Account
}

func NewManager(d db.DB) *Manager {
	am := &Manager{
		store:  d,
		bucket: "ACCOUNT",
	}
	err := am.store.CreateBucket(am.bucket)
	if err != nil {
		log.Fatal(fmt.Errorf("create bucket %s failed: %v", am.bucket, err))
	}
	cache, err := lru.New(10000)
	if err != nil {
		log.Fatal(fmt.Errorf("create account manager LRU cache failed: %v", err))
	}
	am.accounts = cache
	return am
}

// Create master account with native asset (ULT) and initial balances
func (am *Manager) CreateMasterAccount(networkID []byte, balance uint64) error {
	pubKey, privKey, err := crypto.GenerateKeypairFromSeed(networkID)
	if err != nil {
		return err
	}
	log.Infof("master private key (seed) is %s", privKey)

	acc := &ultpb.Account{
		AccountID: pubKey,
		Balance:   balance,
		Signer:    pubKey,
	}
	am.master = acc

	return nil
}

// Get account information from accountID
func (am *Manager) GetAccount(accountID string) (*ultpb.Account, error) {
	// first check the LRU cache
	if acc, ok := am.accounts.Get(accountID); ok {
		return acc.(*ultpb.Account), nil
	}
	// then check database
	b, ok := am.store.Get(am.bucket, []byte(accountID))
	if !ok {
		return nil, fmt.Errorf("account %s not exist", accountID)
	}
	acc, err := ultpb.DecodeAccount(b)
	if err != nil {
		return nil, fmt.Errorf("account %s decode failed: %v", accountID, err)
	}
	// cache the account
	am.accounts.Add(accountID, acc)
	return acc, nil
}
