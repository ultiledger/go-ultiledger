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
	"math/big"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

type RoundType uint8

const (
	RoundUp RoundType = iota
	RoundDown
)

// Compare two prices.
func ComparePrice(lhs *ultpb.Price, rhs *ultpb.Price) int {
	l := big.NewRat(lhs.Numerator, lhs.Denominator)
	r := big.NewRat(rhs.Numerator, rhs.Denominator)

	return l.Cmp(r)
}

// Multiply two int64 numbers.
func MultiplyInt64(lhs int64, rhs int64) *big.Int {
	l := big.NewInt(lhs)
	r := big.NewInt(rhs)

	result := big.NewInt(0)
	result.Mul(l, r)

	return result
}

// Divide the value by an int64 number and round
// the result based on RoundType.
func DivideBigInt(val *big.Int, divisor int64, roundType RoundType) int64 {
	div := big.NewInt(divisor)

	quotient := big.NewInt(0)
	modulus := big.NewInt(0)
	quotient.DivMod(val, div, modulus)

	q := quotient.Int64()
	remainder := modulus.Int64()

	if roundType == RoundUp && remainder > 0 {
		return q + 1
	}
	return q
}
