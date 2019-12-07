// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package account manages the accounts in the network.
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
	ErrBalanceOverflow  = errors.New("account balance overflow")
	ErrBalanceUnderflow = errors.New("account balance underflow")
	ErrBalanceUnderfund = errors.New("account balance underfund")
	ErrTrustOverLimit   = errors.New("trust balance over limit")
	ErrTrustUnderflow   = errors.New("trust balance underflow")
	ErrInvalidUpdate    = errors.New("account update invalid")
)

// Manager manages accounts and trusts.
type Manager struct {
	database db.Database
	bucket   string

	baseReserve int64

	Master *ultpb.Account
}

func NewManager(d db.Database, baseReserve int64) *Manager {
	am := &Manager{
		database:    d,
		bucket:      "ACCOUNT",
		baseReserve: baseReserve,
	}

	err := am.database.NewBucket(am.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", am.bucket, err)
	}

	return am
}

// Create master account with native asset (ULT) and initial balances.
func (am *Manager) CreateMasterAccount(networkID []byte, balance int64, seqNum uint64) error {
	pubKey, privKey, err := crypto.GetAccountKeypairFromSeed(networkID)
	if err != nil {
		return err
	}
	log.Infow("master account created", "seed", privKey, "accountID", pubKey)

	err = am.CreateAccount(am.database, pubKey, balance, pubKey, seqNum)
	if err != nil {
		return fmt.Errorf("create master account failed: %v", err)
	}

	acc, err := am.GetAccount(am.database, pubKey)
	if err != nil {
		return fmt.Errorf("get master account failed: %v", err)
	}
	am.Master = acc

	return nil
}

// Create a new account with initial balance. Note that this method
// simply save the account info in database and all the necessary
// validity checks should be done before invoking this method.
func (am *Manager) CreateAccount(putter db.Putter, accountID string, balance int64, signer string, seqNum uint64) error {
	if balance < am.baseReserve {
		return errors.New("init balance lower than base reserve")
	}

	acc := &ultpb.Account{
		AccountID: accountID,
		Balance:   balance,
		Signer:    signer,
		SeqNum:    seqNum,
		Liability: &ultpb.Liability{
			Selling: int64(0),
			Buying:  int64(0),
		},
	}

	accb, err := ultpb.Encode(acc)
	if err != nil {
		return fmt.Errorf("encode account failed: %v", err)
	}

	// Save the account in db.
	err = putter.Put(am.bucket, []byte(acc.AccountID), accb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	return nil
}

// Get account information.
func (am *Manager) GetAccount(getter db.Getter, accountID string) (*ultpb.Account, error) {
	b, err := getter.Get(am.bucket, []byte(accountID))
	if err != nil {
		return nil, fmt.Errorf("get account %s failed: %v", accountID, err)
	}
	if b == nil {
		return nil, nil
	}

	acc, err := ultpb.DecodeAccount(b)
	if err != nil {
		return nil, fmt.Errorf("account %s decode failed: %v", accountID, err)
	}

	return acc, nil
}

// Update account information.
func (am *Manager) SaveAccount(putter db.Putter, acc *ultpb.Account) error {
	accb, err := ultpb.Encode(acc)
	if err != nil {
		return fmt.Errorf("encode account failed: %v", err)
	}

	err = putter.Put(am.bucket, []byte(acc.AccountID), accb)
	if err != nil {
		return fmt.Errorf("save account in db failed: %v", err)
	}

	return nil
}

// Get the rest limit of native asset the account can have.
func (am *Manager) GetRestLimit(acc *ultpb.Account) int64 {
	return math.MaxInt64 - acc.Balance - acc.Liability.Buying
}

// Get the balance of the account.
func (am *Manager) GetBalance(acc *ultpb.Account) int64 {
	minBalance := int64(acc.EntryCount) * am.baseReserve
	balance := acc.Balance - minBalance - acc.Liability.Selling
	return balance
}

// Update account balance.
func (am *Manager) UpdateBalance(acc *ultpb.Account, balance int64) error {
	if balance > 0 && acc.Balance > math.MaxInt64-balance {
		return ErrBalanceOverflow
	}

	if balance < 0 && acc.Balance < balance {
		return ErrBalanceUnderflow
	}

	acc.Balance += balance

	return nil
}

// Update account liability.
func (am *Manager) UpdateLiability(acc *ultpb.Account, amount int64, buy bool) error {
	if amount == 0 {
		return nil
	}
	if buy {
		if amount > math.MaxInt64-acc.Liability.Buying || amount < -acc.Liability.Buying {
			return ErrInvalidUpdate
		}
		acc.Liability.Buying += amount
	} else {
		if amount > math.MaxInt64-acc.Liability.Selling || amount < -acc.Liability.Selling {
			return ErrInvalidUpdate
		}
		acc.Liability.Selling += amount
	}
	return nil
}

// Update entry count.
func (am *Manager) UpdateEntryCount(acc *ultpb.Account, count int32) error {
	if count == 0 {
		return nil
	}
	totalEntry := acc.EntryCount + count

	balance := int64(totalEntry)*am.baseReserve + acc.Liability.Selling

	if balance > acc.Balance {
		return ErrBalanceUnderfund
	}

	acc.EntryCount = totalEntry

	return nil
}

// Create a new trust for issued asset.
func (am *Manager) CreateTrust(putter db.Putter, accountID string, asset *ultpb.Asset, limit int64) error {
	// self-trust is not necessary
	if accountID == asset.Issuer {
		return nil
	}

	trust := &ultpb.Trust{
		AccountID: accountID,
		Asset:     asset,
		Balance:   0,
		Liability: &ultpb.Liability{
			Buying:  int64(0),
			Selling: int64(0),
		},
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

// Get trust information.
func (am *Manager) GetTrust(getter db.Getter, accountID string, asset *ultpb.Asset) (*ultpb.Trust, error) {
	if accountID == asset.Issuer {
		tst := &ultpb.Trust{
			AccountID:  accountID,
			Asset:      asset,
			Balance:    math.MaxInt64,
			Limit:      math.MaxInt64,
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
	if b == nil {
		return nil, nil
	}

	trust, err := ultpb.DecodeTrust(b)
	if err != nil {
		return nil, fmt.Errorf("decode trust failed: %v", err)
	}

	return trust, nil
}

// Update trust information.
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

// Delete the trust.
func (am *Manager) DeleteTrust(deleter db.Deleter, accountID string, asset *ultpb.Asset) error {
	assetb, err := ultpb.Encode(asset)
	if err != nil {
		return fmt.Errorf("encode asset failed: %v", err)
	}
	key := []byte(accountID)
	key = append(key, assetb...)

	err = deleter.Delete(am.bucket, key)
	if err != nil {
		return fmt.Errorf("delete trust from db failed: %v", err)
	}

	return nil
}

// Get rest limit of custom asset the trust can have.
func (am *Manager) GetTrustRestLimit(trust *ultpb.Trust) int64 {
	return math.MaxInt64 - trust.Balance - trust.Liability.Buying
}

// Get available balance for trust.
func (am *Manager) GetTrustBalance(trust *ultpb.Trust) int64 {
	return trust.Balance - trust.Liability.Selling
}

// Update trust balance.
func (am *Manager) UpdateTrustBalance(trust *ultpb.Trust, balance int64) error {
	if balance > 0 && trust.Balance+balance > trust.Limit {
		return ErrTrustOverLimit
	}
	if balance < 0 && trust.Balance < balance {
		return ErrTrustUnderflow
	}

	trust.Balance += balance

	return nil
}

// Update trust liability.
func (am *Manager) UpdateTrustLiability(trust *ultpb.Trust, amount int64, buy bool) error {
	if amount == 0 {
		return nil
	}
	if buy {
		if amount > math.MaxInt64-trust.Liability.Buying || amount < -trust.Liability.Buying {
			return ErrInvalidUpdate
		}
		trust.Liability.Buying += amount
	} else {
		if amount > math.MaxInt64-trust.Liability.Selling || amount < -trust.Liability.Selling {
			return ErrInvalidUpdate
		}
		trust.Liability.Selling += amount
	}
	return nil
}

// Update entry count.
