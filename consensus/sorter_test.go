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

package consensus

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBallotSorter(t *testing.T) {
	ballots := []*Ballot{
		&Ballot{Value: "ABC", Counter: uint32(1)},
		&Ballot{Value: "ABC", Counter: uint32(2)},
		&Ballot{Value: "BCD", Counter: uint32(3)},
		&Ballot{Value: "CDE", Counter: uint32(3)},
	}
	sort.Sort(BallotSlice(ballots))

	assert.Equal(t, *ballots[0], Ballot{Value: "CDE", Counter: uint32(3)})
	assert.Equal(t, *ballots[1], Ballot{Value: "BCD", Counter: uint32(3)})
	assert.Equal(t, *ballots[2], Ballot{Value: "ABC", Counter: uint32(2)})
	assert.Equal(t, *ballots[3], Ballot{Value: "ABC", Counter: uint32(1)})
}
