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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIntComparison(t *testing.T) {
	assert.Equal(t, 2, MaxInt(1, 2))
	assert.Equal(t, 2, MinInt(3, 2))
}

func TestInt64Comparison(t *testing.T) {
	assert.Equal(t, int64(2), MaxInt64(int64(1), int64(2)))
	assert.Equal(t, int64(2), MinInt64(int64(3), int64(2)))
}

func TestUint64Comparison(t *testing.T) {
	assert.Equal(t, uint64(2), MaxUint64(uint64(1), uint64(2)))
	assert.Equal(t, uint64(2), MinUint64(uint64(3), uint64(2)))
}
