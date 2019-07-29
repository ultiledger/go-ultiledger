package consensus

import (
	"testing"

	"github.com/deckarep/golang-set"
	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestCompareBallots(t *testing.T) {
	// test nil ballots case
	var lBallot, rBallot *Ballot
	assert.Equal(t, 0, compareBallots(lBallot, rBallot))

	// test one nil ballot case
	lBallot = &Ballot{Value: "ABC", Counter: uint32(1)}
	assert.Equal(t, 1, compareBallots(lBallot, rBallot))
	assert.Equal(t, -1, compareBallots(rBallot, lBallot))

	// test ballots with different counter
	rBallot = &Ballot{Value: "ABC", Counter: uint32(2)}
	assert.Equal(t, -1, compareBallots(lBallot, rBallot))

	// test ballots with the same counter
	rBallot.Counter = uint32(1)
	assert.Equal(t, 0, compareBallots(lBallot, rBallot))
	rBallot.Value = "BCD"
	assert.Equal(t, -1, compareBallots(lBallot, rBallot))
}

func TestCompatibleBallots(t *testing.T) {
	// test nil ballots case
	var lBallot, rBallot *Ballot
	assert.Equal(t, false, compatibleBallots(lBallot, rBallot))

	// test one nil ballot case
	lBallot = &Ballot{Value: "ABC", Counter: uint32(1)}
	assert.Equal(t, false, compatibleBallots(lBallot, rBallot))

	// test ballots with the same value
	rBallot = &Ballot{Value: "ABC", Counter: uint32(1)}
	assert.Equal(t, true, compatibleBallots(lBallot, rBallot))

	// test ballots with different value
	rBallot.Value = "BCD"
	assert.Equal(t, false, compatibleBallots(lBallot, rBallot))
}

func TestIsNewerBallot(t *testing.T) {
	// test prepare statement - case 1
	lStmt := &Statement{
		StatementType: ultpb.StatementType_PREPARE,
		Stmt: &ultpb.Statement_Prepare{
			Prepare: &ultpb.Prepare{
				B:  &Ballot{Value: "ABC", Counter: uint32(123)},
				P:  &Ballot{Value: "ABC", Counter: uint32(123)},
				Q:  &Ballot{Value: "ABC", Counter: uint32(123)},
				HC: uint32(1),
			},
		},
	}
	rStmt := &Statement{
		StatementType: ultpb.StatementType_PREPARE,
		Stmt: &ultpb.Statement_Prepare{
			Prepare: &ultpb.Prepare{
				B:  &Ballot{Value: "ABC", Counter: uint32(234)},
				P:  &Ballot{Value: "ABC", Counter: uint32(123)},
				Q:  &Ballot{Value: "ABC", Counter: uint32(123)},
				HC: uint32(1),
			},
		},
	}
	assert.Equal(t, true, isNewerBallot(lStmt, rStmt))
	// test prepare statement - case 2
	lStmt = &Statement{
		StatementType: ultpb.StatementType_PREPARE,
		Stmt: &ultpb.Statement_Prepare{
			Prepare: &ultpb.Prepare{
				B:  &Ballot{Value: "ABC", Counter: uint32(123)},
				P:  &Ballot{Value: "ABC", Counter: uint32(123)},
				Q:  &Ballot{Value: "ABC", Counter: uint32(123)},
				HC: uint32(1),
			},
		},
	}
	rStmt = &Statement{
		StatementType: ultpb.StatementType_PREPARE,
		Stmt: &ultpb.Statement_Prepare{
			Prepare: &ultpb.Prepare{
				B:  &Ballot{Value: "ABC", Counter: uint32(123)},
				P:  &Ballot{Value: "ABC", Counter: uint32(234)},
				Q:  &Ballot{Value: "ABC", Counter: uint32(123)},
				HC: uint32(1),
			},
		},
	}
	assert.Equal(t, true, isNewerBallot(lStmt, rStmt))
	// test prepare statement - case 3
	lStmt = &Statement{
		StatementType: ultpb.StatementType_PREPARE,
		Stmt: &ultpb.Statement_Prepare{
			Prepare: &ultpb.Prepare{
				B:  &Ballot{Value: "ABC", Counter: uint32(123)},
				P:  &Ballot{Value: "ABC", Counter: uint32(123)},
				Q:  &Ballot{Value: "ABC", Counter: uint32(234)},
				HC: uint32(1),
			},
		},
	}
	rStmt = &Statement{
		StatementType: ultpb.StatementType_PREPARE,
		Stmt: &ultpb.Statement_Prepare{
			Prepare: &ultpb.Prepare{
				B:  &Ballot{Value: "ABC", Counter: uint32(123)},
				P:  &Ballot{Value: "ABC", Counter: uint32(123)},
				Q:  &Ballot{Value: "ABC", Counter: uint32(123)},
				HC: uint32(1),
			},
		},
	}
	assert.Equal(t, false, isNewerBallot(lStmt, rStmt))
}

func TestIsProperSubset(t *testing.T) {
	// Case 1: the first set is the proper subset of the second one.
	set1 := []string{"A", "B"}
	set2 := []string{"A", "B", "C"}
	assert.Equal(t, true, isProperSubset(set1, set2))
	// Case 2: the first set is not the proper subset of the second
	// one because of mismatched set.
	set1 = []string{"A", "B", "D"}
	assert.Equal(t, false, isProperSubset(set1, set2))
	// Case 3: the first set is not the proper subset of the second
	// one because of equal set member.
	set1 = []string{"A", "B", "C"}
	assert.Equal(t, false, isProperSubset(set1, set2))
}

func TestIsVBlocking(t *testing.T) {
	// Create a quorum with only first level validators.
	quorum := &ultpb.Quorum{
		Validators: []string{"A", "B", "C", "D", "E"},
		Threshold:  0.5,
	}
	// Case 1: the nodeset forms a v-blocking set.
	nodeset := mapset.NewSet()
	nodeset.Add("A")
	nodeset.Add("B")
	nodeset.Add("C")
	assert.Equal(t, true, isVblocking(quorum, nodeset))
	// Case 2: the nodeset does not form a v-blocking set.
	nodeset.Pop()
	assert.Equal(t, false, isVblocking(quorum, nodeset))
	// Create a quorum with nested quorums.
	quorum = &ultpb.Quorum{
		Validators: []string{"A", "B", "C", "D", "E"},
		Threshold:  0.5,
		NestQuorums: []*ultpb.Quorum{
			&ultpb.Quorum{Validators: []string{"F", "G"}, Threshold: 0.5},
			&ultpb.Quorum{Validators: []string{"H", "I"}, Threshold: 0.5},
		},
	}
	// Case 3: the nodeset forms a v-blocking set with
	// members of the nested quorums.
	nodeset.Clear()
	nodeset.Add("A")
	nodeset.Add("B")
	nodeset.Add("F")
	nodeset.Add("H")
	assert.Equal(t, true, isVblocking(quorum, nodeset))
}

func TestIsQuorumSlice(t *testing.T) {
	// Create a quorum with nested quorums.
	quorum := &ultpb.Quorum{
		Validators: []string{"A", "B", "C", "D", "E"},
		Threshold:  0.5,
		NestQuorums: []*ultpb.Quorum{
			&ultpb.Quorum{Validators: []string{"F", "G"}, Threshold: 0.5},
			&ultpb.Quorum{Validators: []string{"H", "I"}, Threshold: 0.5},
		},
	}
	// Case 1: the nodeset forms a quorum slice
	nodeset := mapset.NewSet()
	nodeset.Add("A")
	nodeset.Add("B")
	nodeset.Add("F")
	nodeset.Add("H")
	assert.Equal(t, true, isQuorumSlice(quorum, nodeset))
	// Case 2 : the nodeset does not form a quorum slice
	nodeset.Clear()
	nodeset.Add("A")
	nodeset.Add("B")
	nodeset.Add("F")
	nodeset.Add("G")
	assert.Equal(t, false, isQuorumSlice(quorum, nodeset))
}

func TestValidateQuorum(t *testing.T) {
	// Create a quorum with nested quorums.
	quorum := &ultpb.Quorum{
		Validators: []string{"A", "B", "C", "D", "E"},
		Threshold:  0.5,
		NestQuorums: []*ultpb.Quorum{
			&ultpb.Quorum{Validators: []string{"F", "G"}, Threshold: 0.5},
			&ultpb.Quorum{Validators: []string{"H", "I"}, Threshold: 0.5},
		},
	}
	assert.Nil(t, ValidateQuorum(quorum, 0, true))
	// Create a quorum with redundant validators.
	quorum.NestQuorums[0].NestQuorums = []*ultpb.Quorum{
		&ultpb.Quorum{Validators: []string{"X", "Y", "Y"}, Threshold: 1.0},
		&ultpb.Quorum{Validators: []string{"M", "N"}, Threshold: 1.0},
	}
	assert.NotNil(t, ValidateQuorum(quorum, 0, true))
	// Create a quorum with deeper depth.
	quorum.NestQuorums[0].NestQuorums = []*ultpb.Quorum{
		&ultpb.Quorum{Validators: []string{"X", "Y"}, Threshold: 1.0},
		&ultpb.Quorum{Validators: []string{"M", "N"}, Threshold: 1.0},
	}
	quorum.NestQuorums[0].NestQuorums[0].NestQuorums = []*ultpb.Quorum{
		&ultpb.Quorum{Validators: []string{"O", "P"}, Threshold: 1.0},
	}
	assert.NotNil(t, ValidateQuorum(quorum, 0, true))
}
