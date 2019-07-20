package consensus

import (
	"math"
	"sort"
	"strings"

	"github.com/deckarep/golang-set"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Type alias for proto types.
type (
	Statement      = ultpb.Statement
	Nominate       = ultpb.Nominate
	Prepare        = ultpb.Prepare
	Confirm        = ultpb.Confirm
	Externalize    = ultpb.Externalize
	Quorum         = ultpb.Quorum
	Ballot         = ultpb.Ballot
	TxSet          = ultpb.TxSet
	ConsensusValue = ultpb.ConsensusValue
)

// Ballots comparison utilities.
func lessAndCompatibleBallots(lb *Ballot, rb *Ballot) bool {
	if compareBallots(lb, rb) <= 0 && compatibleBallots(lb, rb) {
		return true
	}
	return false
}

func lessAndIncompatibleBallots(lb *Ballot, rb *Ballot) bool {
	if compareBallots(lb, rb) <= 0 && !compatibleBallots(lb, rb) {
		return true
	}
	return false
}

// Compare two ballots by counter then value.
func compareBallots(lb *Ballot, rb *Ballot) int {
	// Check input with nil ballots.
	if lb == nil && rb == nil {
		return 0
	} else if lb == nil && rb != nil {
		return -1
	} else if lb != nil && rb == nil {
		return 1
	}

	// First compare the counters.
	if lb.Counter < rb.Counter {
		return -1
	} else if lb.Counter > rb.Counter {
		return 1
	}

	// Then compare the values.
	return strings.Compare(lb.Value, rb.Value)
}

// Check whether the two ballots have the same value.
func compatibleBallots(lb *Ballot, rb *Ballot) bool {
	if lb == nil || rb == nil {
		return false
	}

	if strings.Compare(lb.Value, rb.Value) == 0 {
		return true
	}

	return false
}

// Check whether the second ballot statement is newer than the first one.
func isNewerBallot(lb *Statement, rb *Statement) bool {
	// Compare statetment types in predefined order.
	if lb.StatementType != rb.StatementType {
		return lb.StatementType < rb.StatementType
	}

	switch rb.StatementType {
	case ultpb.StatementType_PREPARE: // Order of ballots: b, p, q, h.
		lp := lb.GetPrepare()
		rp := rb.GetPrepare()
		// Compare working ballots.
		cmp := compareBallots(lp.B, rp.B)
		if cmp < 0 {
			return true
		} else if cmp == 0 {
			// Compare p ballots.
			cmpp := compareBallots(lp.P, rp.P)
			if cmpp < 0 {
				return true
			} else if cmpp == 0 {
				// Compare q ballots.
				cmpq := compareBallots(lp.Q, rp.Q)
				if cmpq < 0 {
					return true
				} else if cmpq == 0 {
					return lp.HC < rp.HC
				}
			}
		}
	case ultpb.StatementType_CONFIRM:
		lc := lb.GetConfirm()
		rc := rb.GetConfirm()
		cmp := compareBallots(lc.B, rc.B)
		if cmp < 0 {
			return true
		} else if cmp == 0 {
			if lc.PC == rc.PC {
				return lc.HC < rc.HC
			}
			return lc.PC < rc.PC
		}
	case ultpb.StatementType_EXTERNALIZE:
		return false
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	return false
}

// Check whether the first set is the proper subset of the second set.
func isProperSubset(a []string, b []string) bool {
	if len(a) > len(b) {
		return false
	}
	as := mapset.NewSet()
	for _, v := range a {
		as.Add(v)
	}
	bs := mapset.NewSet()
	for _, v := range b {
		bs.Add(v)
	}
	if as.IsProperSubset(bs) {
		return true
	}
	return false
}

// Check whether the second nomination contains all the information of the first one.
func isNewerNomination(anom *ultpb.Nominate, bnom *ultpb.Nominate) bool {
	if anom == nil && bnom != nil {
		return true
	}

	if isProperSubset(anom.VoteList, bnom.VoteList) {
		// TODO(bobonovski) more elaborate check like interset?
		return true
	}

	if isProperSubset(anom.AcceptList, bnom.AcceptList) {
		return true
	}

	return false
}

// Get current working ballot.
func getWorkingBallot(stmt *Statement) *Ballot {
	var wb *Ballot

	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		wb = prepare.B
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		wb = &Ballot{Value: confirm.B.Value, Counter: confirm.LC}
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		wb = ext.B
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	return wb
}

// Check whether the input node set form V-blocking for input quorum.
func isVblocking(quorum *ultpb.Quorum, nodeSet mapset.Set) bool {
	qsize := float64(len(quorum.Validators) + len(quorum.NestQuorums))
	threshold := int(math.Ceil(qsize * (1.0 - quorum.Threshold)))

	for _, vid := range quorum.Validators {
		if threshold == 0 {
			return true
		}
		if nodeSet.Contains(vid) {
			threshold = threshold - 1
		}
	}

	for _, nq := range quorum.NestQuorums {
		if threshold == 0 {
			return true
		}
		if isVblocking(nq, nodeSet) {
			threshold = threshold - 1
		}
	}

	return false
}

// Check whether the input node set forms quorum slice for input quorum.
func isQuorumSlice(quorum *ultpb.Quorum, nodeSet mapset.Set) bool {
	qsize := float64(len(quorum.Validators) + len(quorum.NestQuorums))
	threshold := int(math.Ceil(qsize * quorum.Threshold))

	for _, vid := range quorum.Validators {
		if nodeSet.Contains(vid) {
			threshold = threshold - 1
		}
		if threshold <= 0 {
			return true
		}
	}

	for _, nq := range quorum.NestQuorums {
		if isVblocking(nq, nodeSet) {
			threshold = threshold - 1
		}
		if threshold <= 0 {
			return true
		}
	}

	return false
}

// Check whether the quorum is valid.
func isValidQuorum(quorum *ultpb.Quorum, depth int, extraChecks bool) bool {
	// The depth of quorum should not be greater than two.
	if depth > 2 {
		return false
	}
	if quorum.Threshold <= 0.0 || quorum.Threshold > 1.0 {
		return false
	}

	nodes := mapset.NewSet()
	if extraChecks && quorum.Threshold < 1.0-quorum.Threshold {
		return false
	}
	for _, v := range quorum.Validators {
		// duplicate validators are not allowed
		if nodes.Contains(v) {
			return false
		}
		nodes.Add(v)
	}
	for _, q := range quorum.NestQuorums {
		if isValidQuorum(q, depth+1, extraChecks) {
			return false
		}
	}
	return true
}

// Build a quorum with one node.
func getSingletonQuorum(nodeID string) *Quorum {
	quorum := &Quorum{
		Threshold:  1.0,
		Validators: []string{nodeID},
	}
	return quorum
}

// Normalize quorum for the convenience of computing node weights.
func normalizeQuorum(quorum *Quorum, nodeID string) {
	simplifyQuorum(quorum, nodeID)
	sortQuorum(quorum)
}

// Simplify quorum by eliminating unnecessary nesting structures.
func simplifyQuorum(quorum *Quorum, nodeID string) {
	// Remove input node from validators.
	var validators []string
	for _, v := range quorum.Validators {
		if v == nodeID {
			continue
		}
		validators = append(validators, v)
	}
	quorum.Validators = validators
	// Remove input node from the nested quorums.
	for i, _ := range quorum.NestQuorums {
		simplifyQuorum(quorum.NestQuorums[i], nodeID)
	}
	// Flatten unnecessary nesting quorums.
	if quorum.Threshold == 1.0 && len(quorum.Validators) == 0 && len(quorum.NestQuorums) == 1 {
		quorum = quorum.NestQuorums[0]
	}
}

// Sort the quorum by validators then nest quorums.
func sortQuorum(quorum *Quorum) {
	// Sort validators.
	sort.Strings(quorum.Validators)
	// Sort validators of the nest quorums.
	for i, _ := range quorum.NestQuorums {
		sortQuorum(quorum.NestQuorums[i])
	}
	// Sort the nest quorums.
	sort.Sort(QuorumSlice(quorum.NestQuorums))
}

// Get all the nodes from the quorum.
func getQuorumNodes(quorum *Quorum) []string {
	var nodes []string
	for _, v := range quorum.Validators {
		nodes = append(nodes, v)
	}
	for _, q := range quorum.NestQuorums {
		n := getQuorumNodes(q)
		nodes = append(nodes, n...)
	}
	return nodes
}
