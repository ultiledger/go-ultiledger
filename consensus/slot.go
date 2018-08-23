package consensus

import (
	"fmt"
	"math"

	"github.com/deckarep/golang-set"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/db"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

// Slot is responsible for maintaining consensus
// state for a slot index
type Slot struct {
	index  uint64
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	votes       mapset.Set
	accepts     mapset.Set
	candidates  mapset.Set
	nominations map[string]*pb.Nomination
}

func newSlot(idx uint64, d db.DB, l *zap.SugaredLogger) *Slot {
	s := &Slot{
		index:      idx,
		store:      d,
		logger:     l,
		bucket:     "SLOT",
		votes:      mapset.NewSet(),
		accepts:    mapset.NewSet(),
		candidates: mapset.NewSet(),
	}
	return s
}

func (s *Slot) nominate(quorum *pb.Quorum, prevHash, currHash string) (string, error) {
	return "", nil
}

// check whether the input nomination is valid and newer
func (s *Slot) addNomination(nodeID string, newNom *pb.Nomination) error {
	// check validity of votes and accepts
	if len(newNom.VoteList)+len(newNom.AcceptList) == 0 {
		return fmt.Errorf("empty vote and accept list")
	}
	// check whether the existing nomination of the remote node
	// is the proper subset of the new nomination
	if nom, ok := s.nominations[nodeID]; ok {
		if !IsProperSubset(nom.VoteList, newNom.VoteList) {
			// TODO(bobonovski) more elaborate check like interset?
			return fmt.Errorf("old votes is not proper set of new votes")
		}
		if !IsProperSubset(nom.AcceptList, newNom.AcceptList) {
			return fmt.Errorf("old accepts is not proper set of new accepts")
		}
	}
	s.nominations[nodeID] = newNom
	return nil
}

func isVblocking(quorum *pb.Quorum, nodeSet mapset.Set) bool {
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

func isQuorumSlice(quorum *pb.Quorum, nodeSet mapset.Set) bool {
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
func findAcceptNodes(v string, noms map[string]*pb.Nomination) mapset.Set {
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
func findVoteOrAcceptNodes(v string, noms map[string]*pb.Nomination) mapset.Set {
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

// try to promote votes to accepts by checking two conditions:
//   1. whether the votes form V-blocking
//   2. whether all the nodes in the quorum have voted
// then try to promote accepts to candidates by checking:
//   1. whether all the nodes in the quorum have accepted
func (s *Slot) promoteVotes(quorum *pb.Quorum, newNom *pb.Nomination) (bool, bool, error) {
	acceptUpdated := false
	for _, vote := range newNom.VoteList {
		ns := findAcceptNodes(vote, s.nominations)
		// use federated vote to promote value
		if !isVblocking(quorum, ns) {
			nset := findVoteOrAcceptNodes(vote, s.nominations)
			if !isQuorumSlice(quorum, nset) { // TODO(bobonovski) trim nset to contain only other quorums
				return false, false, fmt.Errorf("failed to promote any votes to accepts")
			}
		}
		// TODO(bobonovski) check the validity of the vote
		s.votes.Add(vote)
		s.accepts.Add(vote)
		acceptUpdated = true
	}
	candidateUpdated := false
	for _, accept := range newNom.AcceptList {
		if s.candidates.Contains(accept) {
			continue
		}
		ns := findAcceptNodes(accept, s.nominations)
		if isQuorumSlice(quorum, ns) {
			s.candidates.Add(accept)
			candidateUpdated = true
		}
	}
	return acceptUpdated, candidateUpdated, nil
}
