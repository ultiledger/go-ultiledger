package account

import (
	"errors"

	lru "github.com/hashicorp/golang-lru"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrAccountNotExist  = errors.New("account not exist")
	ErrAccountCorrupted = errors.New("account corrupted")
)

// Manager manages the creation of accounts
type Manager struct {
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	// LRU cache for accounts
	accounts *lru.Cache

	// master account
	master *pb.Account
}

func NewManager(d db.DB, l *zap.SugaredLogger) *Manager {
	am := &Manager{
		store:  d,
		bucket: "ACCOUNTS",
		logger: l,
	}
	err := am.store.CreateBucket(am.bucket)
	if err != nil {
		am.logger.Fatal(err)
	}
	cache, err := lru.New(10000)
	if err != nil {
		am.logger.Fatal(err)
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
	am.logger.Infof("master private key (seed) is %s", privKey)

	acc := &pb.Account{
		AccountID: pubKey,
		Balance:   balance,
		Signer:    pubKey,
	}
	am.master = acc

	return nil
}

// Get account information from accountID
func (am *Manager) GetAccount(accountID string) (*pb.Account, error) {
	// first check the LRU cache
	if acc, ok := am.accounts.Get(accountID); ok {
		return acc.(*pb.Account), nil
	}
	// then check database
	b, ok := am.store.Get(am.bucket, []byte(accountID))
	if !ok {
		return nil, ErrAccountNotExist
	}
	acc, err := pb.DecodeAccount(b)
	if err != nil {
		return nil, ErrAccountCorrupted
	}
	// cache the account
	am.accounts.Add(accountID, acc)
	return acc, nil
}
