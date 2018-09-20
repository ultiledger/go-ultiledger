package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

func TestCompareBallots(t *testing.T) {
	// test nil ballots case
	var lBallot, rBallot *Ballot
	assert.Equal(t, 0, compareBallots(lBallot, rBallot))

	// test one nil ballot case
	lBallot = &Ballot{Value: "ABC", Counter: uint32(1)}
	assert.Equal(t, -1, compareBallots(lBallot, rBallot))
	assert.Equal(t, 1, compareBallots(rBallot, lBallot))

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
