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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestComparePrice(t *testing.T) {
	lhs := &ultpb.Price{
		Numerator:   int64(3),
		Denominator: int64(5),
	}
	rhs := &ultpb.Price{
		Numerator:   int64(4),
		Denominator: int64(7),
	}
	assert.Equal(t, 1, ComparePrice(lhs, rhs))

	lhs.Numerator = int64(1)

	assert.Equal(t, -1, ComparePrice(lhs, rhs))

	lhs.Numerator = int64(8)
	lhs.Denominator = int64(14)

	assert.Equal(t, 0, ComparePrice(lhs, rhs))
}
