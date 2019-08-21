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

package exchange

import "github.com/ultiledger/go-ultiledger/ultpb"

// Sort offers in ascending order by price.
type OfferSlice []*ultpb.Offer

func (os OfferSlice) Len() int {
	return len(os)
}

func (os OfferSlice) Less(i, j int) bool {
	cmp := ComparePrice(os[i].Price, os[j].Price)
	if cmp <= 0 {
		return true
	}
	return false
}

func (os OfferSlice) Swap(i, j int) {
	os[i], os[j] = os[j], os[i]
}
