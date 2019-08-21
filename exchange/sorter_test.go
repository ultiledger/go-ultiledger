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

import (
	"sort"
	"testing"

	"github.com/ultiledger/go-ultiledger/ultpb"

	"github.com/stretchr/testify/assert"
)

func TestOfferSorter(t *testing.T) {
	offers := []*ultpb.Offer{
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(3)}},
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(4)}},
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(5)}},
		&ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(4)}},
	}
	sort.Sort(OfferSlice(offers))

	assert.Equal(t, *offers[0], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(5)}})
	assert.Equal(t, *offers[1], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(3), Denominator: int64(4)}})
	assert.Equal(t, *offers[2], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(4)}})
	assert.Equal(t, *offers[3], ultpb.Offer{Price: &ultpb.Price{Numerator: int64(4), Denominator: int64(3)}})
}
