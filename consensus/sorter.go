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

import "strings"

// Custom sorter for ballot slice.
type BallotSlice []*Ballot

// Return the length of underlying ballot slice.
func (bs BallotSlice) Len() int {
	return len(bs)
}

// Sort ballots in descending order by comparing.
// counter first then value
func (bs BallotSlice) Less(i, j int) bool {
	if bs[i].Counter > bs[j].Counter {
		return true
	} else if bs[i].Counter == bs[j].Counter {
		if strings.Compare(bs[i].Value, bs[j].Value) >= 0 {
			return true
		}
	}
	return false
}

func (bs BallotSlice) Swap(i, j int) {
	bs[i], bs[j] = bs[j], bs[i]
}

// Custom sorter for quorum.
type QuorumSlice []*Quorum

// Returns the length of underlying quorum slice.
func (qs QuorumSlice) Len() int {
	return len(qs)
}

// Sort quorums by lexicographical order of validators
// then by the lexicographical order of nest quorums.
func (qs QuorumSlice) Less(i, j int) bool {
	cmp := compareQuorum(qs[i], qs[j])
	if cmp <= 0 {
		return true
	}
	return false
}

func (qs QuorumSlice) Swap(i, j int) {
	qs[i], qs[j] = qs[j], qs[i]
}

func compareQuorum(q1 *Quorum, q2 *Quorum) int {
	cmp := compareStrings(q1.Validators, q1.Validators)
	if cmp != 0 {
		return cmp
	}
	// validators are the same and need to compare nest quorums
	cmp = compareQuorums(q1.NestQuorums, q2.NestQuorums)
	if cmp != 0 {
		return cmp
	}
	if q1.Threshold < q2.Threshold {
		return -1
	} else if q1.Threshold > q2.Threshold {
		return 1
	}
	return 0
}

func compareQuorums(qa []*Quorum, qb []*Quorum) int {
	lena, lenb := len(qa), len(qb)
	for i := 0; i < lena && i < lenb; i++ {
		cmp := compareQuorum(qa[i], qb[i])
		if cmp != 0 {
			return cmp
		}
	}
	if lena < lenb {
		return -1
	} else if lena > lenb {
		return 1
	}
	return 0

}

func compareStrings(a []string, b []string) int {
	lena, lenb := len(a), len(b)
	for i := 0; i < lena && i < lenb; i++ {
		cmp := strings.Compare(a[i], b[i])
		if cmp != 0 {
			return cmp
		}
	}
	if lena < lenb {
		return -1
	} else if lena > lenb {
		return 1
	}
	return 0
}
