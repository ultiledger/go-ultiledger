package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/log"
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

	// Use the first node as the local node.
	localNode := nodes[0]

	// Create a quorum.
	quorum := getTestFlatQuorum(nodes, 0.5)
	assert.Equal(t, 3, len(quorum.Validators))

	// Get the quorum hash.
	quorumHash, _ := ultpb.SHA256Hash(quorum)

	// Create a decree.
	d := &Decree{
		index:      1,
		nodeID:     localNode,
		quorum:     quorum,
		quorumHash: quorumHash,
	}

	// Test leader update.
	d.updateRoundLeaders()
	assert.Equal(t, 3, len(quorum.Validators))

	// Get the histrogram of the leaders.
	hist := make(map[string]int)
	for i := 0; i < 100; i++ {
		d.nominationRound = i
		d.updateRoundLeaders()
		hist[d.nominationLeaders[0]] += 1
	}
	log.Info(hist)
}
