package account

import (
	"errors"
	"fmt"
	"math"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrAccountNotExist  = errors.New("account not exist")
	ErrBalanceOverflow  = errors.New("account balance overflow")
	ErrBalanceUnderflow = errors.New("account balance underflow")
	ErrBalanceUnderfund = errors.New("account balance underfund")
	ErrTrustOverLimit   = errors.New("trust balance over limit")
	ErrTrustUnderflow   = errors.New("trust balance underflow")
)

// Manager manages the creation of accounts
type Manager struct {
	database db.Database
	bucket   string

	// master account
	master *ultpb.Account

	baseReserve uint64
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

	return nil
}

// Get account information from accountID
func (am *Manager) GetAccount(getter db.Getter, accountID string) (*ultpb.Account, error) {
	// get account from database
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

	return acc, nil
}

// Update account information
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

// Get the rest limit of native asset the account can have
func (am *Manager) GetRestLimit(acc *ultpb.Account) uint64 {
	return math.MaxUint64 - acc.Balance - acc.Liability.Buying
}

// Get the available balance of the account
func (am *Manager) GetBalance(acc *ultpb.Account) uint64 {
	minBalance := uint64(acc.EntryCount) * am.baseReserve
	balance := acc.Balance - minBalance - acc.Liability.Selling
	return balance
}

// Add balance to account and check balance overflow
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

// Increase entry count and check sufficiency of balance
func (am *Manager) AddEntryCount(acc *ultpb.Account, count uint32) error {
	if count == 0 {
		return nil
	}
	totalEntry := acc.EntryCount + count

	balance := uint64(totalEntry)*am.baseReserve + acc.Liability.Selling

	if balance > acc.Balance {
		return ErrBalanceUnderfund
	}

	acc.EntryCount = totalEntry

	return nil
}

// Decrease entry count
func (am *Manager) SubEntryCount(acc *ultpb.Account, count uint32) error {
	if count == 0 {
		return nil
	}
	totalEntry := acc.EntryCount - count

	balance := uint64(totalEntry)*am.baseReserve + acc.Liability.Selling

	// this should not happen since we are substracting entry count
	if balance > acc.Balance {
		return ErrBalanceUnderfund
	}

	acc.EntryCount = totalEntry

	return nil
}

// Create a new trust for issued asset
func (am *Manager) CreateTrust(putter db.Putter, accountID string, asset *ultpb.Asset, limit uint64) error {
	// self-trust is not necessary
	if accountID == asset.Issuer {
		return nil
	}

	trust := &ultpb.Trust{
		AccountID:  accountID,
		Asset:      asset,
		Balance:    0,
		Limit:      limit,
		Authorized: 1,
	}

	trustb, err := ultpb.Encode(trust)
	if err != nil {
		return fmt.Errorf("encode trust failed: %v", err)
	}

	// construct db key
	assetb, err := ultpb.Encode(asset)
	if err != nil {
		return fmt.Errorf("encode asset failed: %v", err)
	}
	key := []byte(accountID)
	key = append(key, assetb...)

	// save the trust in db
	err = putter.Put(am.bucket, key, trustb)
	if err != nil {
		return fmt.Errorf("save trust in db failed: %v", err)
	}

	return nil
}

// Get trust information
func (am *Manager) GetTrust(getter db.Getter, accountID string, asset *ultpb.Asset) (*ultpb.Trust, error) {
	if accountID == asset.Issuer {
		tst := &ultpb.Trust{
			AccountID:  accountID,
			Asset:      asset,
			Balance:    math.MaxUint64,
			Limit:      math.MaxUint64,
			Authorized: 1,
		}
		return tst, nil
	}

	// construct db key
	assetb, err := ultpb.Encode(asset)
	if err != nil {
		return nil, fmt.Errorf("encode asset failed: %v", err)
	}
	key := []byte(accountID)
	key = append(key, assetb...)

	// get trust in db
	b, err := getter.Get(am.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("get trust from db failed: %v", err)
	}

	trust, err := ultpb.DecodeTrust(b)
	if err != nil {
		return nil, fmt.Errorf("decode trust failed: %v", err)
	}

	return trust, nil
}

// Update trust information
func (am *Manager) SaveTrust(putter db.Putter, trust *ultpb.Trust) error {
	trustb, err := ultpb.Encode(trust)
	if err != nil {
		return fmt.Errorf("encode account failed: %v", err)
	}

	// construct db key
	assetb, err := ultpb.Encode(trust.Asset)
	if err != nil {
		return fmt.Errorf("encode asset failed: %v", err)
	}
	key := []byte(trust.AccountID)
	key = append(key, assetb...)

	// update account in db
	err = putter.Put(am.bucket, key, trustb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	return nil
}

// Get rest limit of custom asset the trust can have
func (am *Manager) GetTrustRestLimit(trust *ultpb.Trust) uint64 {
	return math.MaxUint64 - trust.Balance - trust.Liability.Buying
}

// Get available balance for trust
func (am *Manager) GetTrustBalance(trust *ultpb.Trust) uint64 {
	return trust.Balance - trust.Liability.Selling
}

// Add balance to trust and check out of limit
func (am *Manager) AddTrustBalance(trust *ultpb.Trust, balance uint64) error {
	if trust.Balance+balance > trust.Limit {
		return ErrTrustOverLimit
	}

	trust.Balance += balance

	return nil
}

// Substract balance from trust and check balance underfund
func (am *Manager) SubTrustBalance(trust *ultpb.Trust, balance uint64) error {
	if trust.Balance < balance {
		return ErrTrustUnderflow
	}

	trust.Balance -= balance

	return nil
}

// Get offer from database
func (am *Manager) GetOffer(getter db.Getter, offerID string) (*ultpb.Offer, error) {
	// get offer in db
	b, err := getter.Get(am.bucket, []byte(offerID))
	if err != nil {
		return nil, fmt.Errorf("get offer from db failed: %v", err)
	}

	offer, err := ultpb.DecodeOffer(b)
	if err != nil {
		return nil, fmt.Errorf("decode offer failed: %v", err)
	}

	if offer.OfferID != offerID {
		return nil, errors.New("offerID is incompatible")
	}

	return offer, nil
}

// Update offer in database
func (am *Manager) SaveOffer(putter db.Putter, offer *ultpb.Offer) error {
	offerb, err := ultpb.Encode(offer)
	if err != nil {
		return fmt.Errorf("encode offer failed: %v", err)
	}

	// update offer in db
	err = putter.Put(am.bucket, []byte(offer.OfferID), offerb)
	if err != nil {
		return fmt.Errorf("save offer in db failed: %v", err)
	}

	return nil
}

// Delete offer in database
func (am *Manager) DeleteOffer(deleter db.Deleter, offerID string) error {
	err := deleter.Delete(am.bucket, []byte(offerID))
	if err != nil {
		return fmt.Errorf("deleter offer in db failed: %v", err)
	}

	return nil
}
