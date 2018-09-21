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
