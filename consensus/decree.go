package consensus

import (
	"errors"
	"fmt"
	"math"

	"github.com/deckarep/golang-set"

	"github.com/ultiledger/go-ultiledger/ultpb"
)

type BallotState uint8

const (
	BallotStatePrepare BallotState = iota
	BallotStateConfirm
	BallotStateExternalize
)

// Decree is an abstractive decision the consensus engine
// should reach in each round
type Decree struct {
	index      uint64
	nodeID     string
	quorum     *ultpb.Quorum
	quorumHash string

	// nomination round
	round int

	// latest nomination
	latestNomination *ultpb.Nomination

	// latest composite candidate
	latestComposite string

	// for nominations
	votes       mapset.Set
	accepts     mapset.Set
	candidates  mapset.Set
	nominations map[string]*ultpb.Nomination

	// channel for sending statements
	statementChan chan *ultpb.Statement
}

func NewDecree(idx uint64, nodeID string, quorum *ultpb.Quorum, quorumHash string, stmtC chan *ultpb.Statement) *Decree {
	d := &Decree{
		index:         idx,
		nodeID:        nodeID,
		quorum:        quorum,
		quorumHash:    quorumHash,
		round:         0,
		votes:         mapset.NewSet(),
		accepts:       mapset.NewSet(),
		candidates:    mapset.NewSet(),
		nominations:   make(map[string]*ultpb.Nomination),
		statementChan: stmtC,
	}
	return d
}

// Nominate nominates a consensus value for the decree
func (d *Decree) Nominate(prevHash, currHash string) error {
	d.round++
	// TODO(bobonovski) compute leader weights
	d.votes.Add(currHash) // For test

	if err := d.sendNomination(); err != nil {
		return fmt.Errorf("send nomination failed: %v", err)
	}
	return nil
}

// Recv receives validated statement and redistributes it to
// corresponding route handler
func (d *Decree) Recv(stmt *ultpb.Statement) error {
	if stmt == nil {
		return nil
	}

	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		nom, err := ultpb.DecodeNomination(stmt.Data)
		if err != nil {
			return fmt.Errorf("decode nomination failed: %v", err)
		}
		err = d.recvNomination(d.nodeID, nom)
		if err != nil {
			return fmt.Errorf("recv nomination failed: %v", err)
		}
	}
	return nil
}

// receive nomination from peers or local node
func (d *Decree) recvNomination(nodeID string, nom *ultpb.Nomination) error {
	d.addNomination(nodeID, nom)
	acceptUpdated, candidateUpdated, err := d.promoteVotes(nom)
	if err != nil {
		return fmt.Errorf("promote votes failed: %v", err)
	}

	// send new nomination if votes changed
	if acceptUpdated {
		d.sendNomination()
	}

	// start balloting if candidates changed
	if candidateUpdated {
		compValue, err := d.combineCandidates()
		if err != nil {
			return fmt.Errorf("combine candidates failed: %v", err)
		}
		d.latestComposite = compValue
		// TODO(bobonovski) balloting
	}

	return nil
}

// assemble a nomination and broadcast it to other peers
func (d *Decree) sendNomination() error {
	// create an abstract nomination statement
	nom := &ultpb.Nomination{
		QuorumHash: d.quorumHash,
	}
	for vote := range d.votes.Iter() {
		nom.VoteList = append(nom.VoteList, vote.(string))
	}
	for accept := range d.accepts.Iter() {
		nom.AcceptList = append(nom.AcceptList, accept.(string))
	}

	if err := d.recvNomination(d.nodeID, nom); err != nil {
		return fmt.Errorf("receive local nomination failed: %v", err)
	}

	// broadcast the nomination if it is a new one
	if isNewerNomination(d.latestNomination, nom) {
		d.latestNomination = nom
		nomBytes, err := ultpb.Encode(nom)
		if err != nil {
			return fmt.Errorf("encode nomination failed: %v", err)
		}
		stmt := &ultpb.Statement{
			StatementType: ultpb.StatementType_NOMINATE,
			NodeID:        d.nodeID,
			Index:         d.index,
			Data:          nomBytes,
		}
		d.statementChan <- stmt
	}

	return nil
}

// check whether the input nomination is valid and newer
func (d *Decree) addNomination(nodeID string, newNom *ultpb.Nomination) error {
	// check validity of votes and accepts
	if len(newNom.VoteList)+len(newNom.AcceptList) == 0 {
		return errors.New("vote and accept list is empty")
	}

	// check whether the existing nomination of the remote node
	// is the proper subset of the new nomination
	if nom, ok := d.nominations[nodeID]; ok {
		if isNewerNomination(nom, newNom) {
			d.nominations[nodeID] = newNom
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

// try to promote votes to accepts by checking two conditions (ACCEPT):
//   1. whether the votes form V-blocking
//   2. whether all the nodes in the quorum have voted
// then try to promote accepts to candidates by checking (CONFIRM):
//   1. whether all the nodes in the quorum have accepted
func (d *Decree) promoteVotes(newNom *ultpb.Nomination) (bool, bool, error) {
	acceptUpdated := false
	for _, vote := range newNom.VoteList {
		if d.accepts.Contains(vote) {
			continue
		}

		// use federated vote to promote value
		ns := findAcceptNodes(vote, d.nominations)
		if !isVblocking(d.quorum, ns) {
			nset := findVoteOrAcceptNodes(vote, d.nominations)
			if !isQuorumSlice(d.quorum, nset) { // TODO(bobonovski) trim nset to contain only other quorums
				return false, false, fmt.Errorf("failed to promote any votes to accepts")
			}
		}

		// TODO(bobonovski) check the validity of the vote
		d.votes.Add(vote)
		d.accepts.Add(vote)
		acceptUpdated = true
	}

	candidateUpdated := false
	for _, accept := range newNom.AcceptList {
		if d.candidates.Contains(accept) {
			continue
		}

		ns := findAcceptNodes(accept, d.nominations)
		if isQuorumSlice(d.quorum, ns) {
			d.candidates.Add(accept)
			candidateUpdated = true
		}
	}

	return acceptUpdated, candidateUpdated, nil
}

func (d *Decree) combineCandidates() (string, error) {
	return "", nil
}
