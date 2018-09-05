package consensus

import (
	"math"
	"strings"

	"github.com/deckarep/golang-set"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// compare two ballots by counter then value
func compareBallots(lb *ultpb.Ballot, rb *ultpb.Ballot) int {
	// check input with nil ballot
	if lb == nil && rb == nil {
		return 0
	} else if lb == nil && rb != nil {
		return 1
	} else if lb != nil && rb == nil {
		return -1
	}

	// check normal case
	if lb.Counter < rb.Counter {
		return -1
	} else if lb.Counter > rb.Counter {
		return 1
	}

	return strings.Compare(lb.Value, rb.Value)
}

// check whether the two ballots has the same value
func compatibleBallots(lb *ultpb.Ballot, rb *ultpb.Ballot) bool {
	if lb == nil || rb == nil {
		return false
	}

	if strings.Compare(lb.Value, rb.Value) == 0 {
		return true
	}

	return false
}

// check whether the latter ballot statement is newer than first one
func isNewerBallot(lb *ultpb.Statement, rb *ultpb.Statement) bool {
	return false
}

// Check whether the first set is the proper subset of the second subset
func IsProperSubset(a []string, b []string) bool {
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

// check whether the latter nomination contains all the information of the first one
func isNewerNomination(anom *ultpb.Nomination, bnom *ultpb.Nomination) bool {
	if anom == nil && bnom != nil {
		return true
	}

	if !IsProperSubset(anom.VoteList, bnom.VoteList) {
		// TODO(bobonovski) more elaborate check like interset?
		return false
	}

	if !IsProperSubset(anom.AcceptList, bnom.AcceptList) {
		return false
	}

	return true
}

// check whether the input node set form V-blocking for input quorum
func isVblocking(quorum *ultpb.Quorum, nodeSet mapset.Set) bool {
	qsize := float64(len(quorum.Validators) + len(quorum.NestQuorums))
	threshold := int(math.Ceil(qsize * (1.0 - quorum.Threshold)))

	for _, vid := range quorum.Validators {
		if nodeSet.Contains(vid) {
			threshold = threshold - 1
		}
		if threshold == 0 {
			return true
		}
	}

	for _, nq := range quorum.NestQuorums {
		if isVblocking(nq, nodeSet) {
			threshold = threshold - 1
		}
		if threshold == 0 {
			return true
		}
	}

	return false
}

// check whether the input node set form quorum slice for input quorum
func isQuorumSlice(quorum *ultpb.Quorum, nodeSet mapset.Set) bool {
	qsize := float64(len(quorum.Validators) + len(quorum.NestQuorums))
	threshold := int(math.Ceil(qsize * quorum.Threshold))

	for _, vid := range quorum.Validators {
		if nodeSet.Contains(vid) {
			threshold = threshold - 1
		}
		if threshold == 0 {
			return true
		}
	}

	for _, nq := range quorum.NestQuorums {
		if isVblocking(nq, nodeSet) {
			threshold = threshold - 1
		}
		if threshold == 0 {
			return true
		}
	}

	return false
}

// find set of nodes claimed to accept the value
func findAcceptNodes(v string, noms map[string]*ultpb.Nomination) mapset.Set {
	nodeSet := mapset.NewSet()

	for k, nom := range noms {
		for _, av := range nom.AcceptList {
			if v == av {
				nodeSet.Add(k)
				break
			}
		}
	}

	return nodeSet
}

// find set of nodes claimed to vote or accept the value
func findVoteOrAcceptNodes(v string, noms map[string]*ultpb.Nomination) mapset.Set {
	nodeSet := mapset.NewSet()

	for k, nom := range noms {
		for _, vv := range nom.VoteList {
			if v == vv {
				nodeSet.Add(k)
				break
			}
		}
		for _, av := range nom.AcceptList {
			if v == av {
				nodeSet.Add(k)
				break
			}
		}
	}

	return nodeSet
}
