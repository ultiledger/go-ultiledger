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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Create a quorum with flat structure.
func getTestFlatQuorum(nodeids []string, threshold float64) *ultpb.Quorum {
	q := &ultpb.Quorum{
		Threshold:  threshold,
		Validators: nodeids,
	}
	return q
}

func TestLeaderUpdate(t *testing.T) {
	// Create test nodes.
	var nodes []string
	for i := 0; i < 3; i++ {
		pk, _, _ := crypto.GetNodeKeypair()
		nodes = append(nodes, pk)
	}

	// Create a quorum.
	quorum := getTestFlatQuorum(nodes, 0.6)
	assert.Equal(t, 3, len(quorum.Validators))

	// Get the quorum hash.
	quorumHash, _ := ultpb.SHA256Hash(quorum)

	// Create multiple decrees to simulate the decree of each node.
	var decrees []*Decree
	for i := 0; i < 3; i++ {
		d := &Decree{
			index:      1,
			nodeID:     nodes[i],
			quorum:     quorum,
			quorumHash: quorumHash,
		}
		decrees = append(decrees, d)
	}

	// Get the histograms of the leaders.
	var hists []map[string]int
	for i := 0; i < 3; i++ {
		hists = append(hists, make(map[string]int))
	}
	for k := 0; k < 100; k++ {
		var leaders []string
		for i := 0; i < 3; i++ {
			decrees[i].nominationRound = k
			decrees[i].updateRoundLeaders()
			hists[i][decrees[i].nominationLeaders[0]] += 1
			leaders = append(leaders, decrees[i].nominationLeaders[0])
		}
		fmt.Println(leaders)
	}
	for i := 0; i < 3; i++ {
		fmt.Printf("Leader histograms of node %s: \n", nodes[i])
		for n, c := range hists[i] {
			fmt.Printf("  %s %d\n", n, c)
		}
	}
}
