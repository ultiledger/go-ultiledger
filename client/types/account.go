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

package types

// Account represents a Ultiledger account in the Ultiledger network.
type Account struct {
	// Public key of this account.
	AccountID string
	// The account balance in ULU.
	Balance int64
	// Public key of the signer of the account.
	Signer string
	// Latest transaction sequence number.
	SeqNum uint64
	// Number of entries belong to the account.
	EntryCount int32
	// Buying liability of native asset.
	BuyingLiability int64
	// Selling liability of native asset.
	SellingLiability int64
}
