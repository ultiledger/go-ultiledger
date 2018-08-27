package consensus

import (
	"fmt"
	"math"

	"github.com/deckarep/golang-set"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Slot is responsible for maintaining consensus
// state for a slot index
type Slot struct {
	index uint64

	logger *zap.SugaredLogger

	// nodeID of this node
	nodeID string

	// nomination round
	round int

	// latest nomination
	latestNomination *ultpb.Nomination

	votes       mapset.Set
	accepts     mapset.Set
	candidates  mapset.Set
	nominations map[string]*ultpb.Nomination

	// channel for sending statements
	statementChan chan *ultpb.Statement
}

func newSlot(idx uint64, nodeID string, l *zap.SugaredLogger) *Slot {
	s := &Slot{
		index:      idx,
		logger:     l,
		nodeID:     nodeID,
		round:      0,
		votes:      mapset.NewSet(),
		accepts:    mapset.NewSet(),
		candidates: mapset.NewSet(),
	}
	return s
}

// nominate a consensus value for this slot
func (s *Slot) Nominate(quorum *ultpb.Quorum, quorumHash, prevHash, currHash string) error {
	s.round++
	// TODO(bobonovski) compute leader weights
	s.votes.Add(currHash) // For test

	if err := s.sendNomination(quorum, quorumHash); err != nil {
		return err
	}
	return nil
}

// receive nomination from peers or local node
func (s *Slot) RecvNomination(nodeID string, quorum *ultpb.Quorum, quorumHash string, nom *ultpb.Nomination) error {
	s.addNomination(s.nodeID, nom)
	acceptUpdated, candidateUpdated, err := s.promoteVotes(quorum, nom)
	if err != nil {
		return err
	}
	// send new nomination if votes changed
	if acceptUpdated {
		s.sendNomination(quorum, quorumHash)
	}
	// start balloting if candidates changed
	if candidateUpdated {
		// TODO(bobonovski) balloting
	}
	return nil
}

// assemble a nomination and broadcast it to other peers
func (s *Slot) sendNomination(quorum *ultpb.Quorum, quorumHash string) error {
	// create an abstract nomination statement
	nom := &ultpb.Nomination{
		QuorumHash: quorumHash,
	}
	for vote := range s.votes.Iter() {
		nom.VoteList = append(nom.VoteList, vote.(string))
	}
	for accept := range s.accepts.Iter() {
		nom.AcceptList = append(nom.AcceptList, accept.(string))
	}
	if err := s.RecvNomination(s.nodeID, quorum, quorumHash, nom); err != nil {
		s.logger.Warnf("failed to accept local nomination: %v", err)
		return err
	}
	// broadcast the nomination if it is a new one
	if isNewerNomination(s.latestNomination, nom) {
		s.latestNomination = nom
		nomBytes, err := ultpb.Encode(nom)
		if err != nil {
			return err
		}
		stmt := &ultpb.Statement{
			StatementType: ultpb.Statement_NOMINATE,
			NodeID:        s.nodeID,
			SlotIndex:     s.index,
			Data:          nomBytes,
		}
		s.statementChan <- stmt
	}
	return nil
}

// check whether the input nomination is valid and newer
func (s *Slot) addNomination(nodeID string, newNom *ultpb.Nomination) error {
	// check validity of votes and accepts
	if len(newNom.VoteList)+len(newNom.AcceptList) == 0 {
		return fmt.Errorf("empty vote and accept list")
	}
	// check whether the existing nomination of the remote node
	// is the proper subset of the new nomination
	if nom, ok := s.nominations[nodeID]; ok {
		if isNewerNomination(nom, newNom) {
			s.nominations[nodeID] = newNom
		}
	}
	return nil
}

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

// try to promote votes to accepts by checking two conditions:
//   1. whether the votes form V-blocking
//   2. whether all the nodes in the quorum have voted
// then try to promote accepts to candidates by checking:
//   1. whether all the nodes in the quorum have accepted
func (s *Slot) promoteVotes(quorum *ultpb.Quorum, newNom *ultpb.Nomination) (bool, bool, error) {
	acceptUpdated := false
	for _, vote := range newNom.VoteList {
		if s.accepts.Contains(vote) {
			continue
		}
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
