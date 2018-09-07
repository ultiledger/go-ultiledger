package consensus

import (
	"strings"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Vote filter to choose nominate statement that has voted the input vote value
func voteFilter(vote string) func(*Statement) bool {
	return func(s *Statement) bool {
		nom := s.GetNominate()
		for _, v := range nom.VoteList {
			if strings.Compare(vote, v) == 0 {
				return true
			}
		}
		return false
	}
}

// Accept filter to choose statement that has accepted the input vote value
func acceptFilter(vote string) func(*Statement) bool {
	return func(s *Statement) bool {
		nom := s.GetNominate()
		for _, v := range nom.AcceptList {
			if strings.Compare(vote, v) == 0 {
				return true
			}
		}
		return false
	}
}

// Vote filter to choose ballot statement that has voted the the input ballot
func ballotVoteFilter(b *Ballot) func(*Statement) bool {
	return func(stmt *Statement) bool {
		if stmt == nil {
			return false
		}
		switch stmt.StatementType {
		case ultpb.StatementType_PREPARE:
			prepare := stmt.GetPrepare()
			if lessAndCompatibleBallots(b, prepare.B) {
				return true
			}
		case ultpb.StatementType_CONFIRM:
			confirm := stmt.GetConfirm()
			if compatibleBallots(b, confirm.B) {
				return true
			}
		case ultpb.StatementType_EXTERNALIZE:
			ext := stmt.GetExternalize()
			if compatibleBallots(b, ext.B) {
				return true
			}
		default:
			log.Fatalf("invalid ballot statement type: %d", stmt.StatementType)
		}
		return false
	}
}

// Accept filter to choose ballot statement that has accepted the input ballot,
// in other words, has accepted prepared of the ballot.
func ballotAcceptFilter(b *Ballot) func(*Statement) bool {
	return func(stmt *Statement) bool {
		if stmt == nil {
			return false
		}
		switch stmt.StatementType {
		case ultpb.StatementType_PREPARE:
			prepare := stmt.GetPrepare()
			if prepare.P != nil && lessAndCompatibleBallots(b, prepare.P) {
				return true
			}
			if prepare.Q != nil && lessAndCompatibleBallots(b, prepare.Q) {
				return true
			}
		case ultpb.StatementType_CONFIRM:
			confirm := stmt.GetConfirm()
			tmp := &Ballot{Value: confirm.B.Value, Counter: confirm.PC}
			if lessAndCompatibleBallots(b, tmp) {
				return true
			}
		case ultpb.StatementType_EXTERNALIZE:
			ext := stmt.GetExternalize()
			if compatibleBallots(b, ext.B) {
				return true
			}
		default:
			log.Fatalf("invalide ballot statement type: %d", stmt.StatementType)
		}
		return false
	}
}
