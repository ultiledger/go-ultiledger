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
	"strings"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Vote filter to choose nominate statements that have voted the input vote value.
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

// Accept filter to choose statements that have accepted the input vote value.
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

// Vote filter to choose ballot statements that has voted the the prepare ballot.
func prepareVoteFilter(b *Ballot) func(*Statement) bool {
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

// Accept filter to choose ballot statements that have accepted the prepare ballot.
func prepareAcceptFilter(b *Ballot) func(*Statement) bool {
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
			log.Fatal(ErrUnknownStmtType)
		}
		return false
	}
}

// Vote filter to choose ballot statements that have voted the commit ballot.
func commitVoteFilter(b *Ballot, l uint32, r uint32) func(*Statement) bool {
	return func(stmt *Statement) bool {
		if stmt == nil {
			return false
		}
		cond := false
		switch stmt.StatementType {
		case ultpb.StatementType_PREPARE:
			prepare := stmt.GetPrepare()
			if compatibleBallots(b, prepare.B) {
				if prepare.LC != 0 {
					cond = prepare.LC <= l && r <= prepare.HC
				}
			}
		case ultpb.StatementType_CONFIRM:
			confirm := stmt.GetConfirm()
			if compatibleBallots(b, confirm.B) {
				cond = confirm.LC <= l
			}
		case ultpb.StatementType_EXTERNALIZE:
			ext := stmt.GetExternalize()
			if compatibleBallots(b, ext.B) {
				cond = ext.B.Counter <= l
			}
		default:
			log.Fatal(ErrUnknownStmtType)
		}
		return cond
	}
}

// Accept filter to choose ballot statements that have accepted the commit ballot.
func commitAcceptFilter(b *Ballot, l uint32, r uint32) func(*Statement) bool {
	return func(stmt *Statement) bool {
		if stmt == nil {
			return false
		}
		cond := false
		switch stmt.StatementType {
		case ultpb.StatementType_PREPARE:
		case ultpb.StatementType_CONFIRM:
			confirm := stmt.GetConfirm()
			if compatibleBallots(b, confirm.B) {
				cond = confirm.LC <= l && r <= confirm.HC
			}
		case ultpb.StatementType_EXTERNALIZE:
			ext := stmt.GetExternalize()
			if compatibleBallots(b, ext.B) {
				cond = ext.B.Counter <= l
			}
		default:
			log.Fatal(ErrUnknownStmtType)
		}
		return cond
	}
}
