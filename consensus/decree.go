package consensus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/deckarep/golang-set"
	pb "github.com/golang/protobuf/proto"
	b58 "github.com/mr-tron/base58/base58"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrUnknownStmtType = errors.New("unknown statement type")
)

type BallotPhase uint8

const (
	BallotPhasePrepare BallotPhase = iota
	BallotPhaseConfirm
	BallotPhaseExternalize
)

// Information about the externalized value.
type ExternalizeValue struct {
	Index uint64 // index of decree
	Value string // consensus value
}

// DecreeContext contains contextual information Decree needs.
type DecreeContext struct {
	Index           uint64                   // decree index
	NodeID          string                   // local node ID
	Quorum          *Quorum                  // local node quorum
	QuorumHash      string                   // local node quorum hash
	StmtChan        chan<- *Statement        // channel for broadcasting statement
	ExternalizeChan chan<- *ExternalizeValue // channel for notifying externlized value
	LM              *ledger.Manager
	Validator       *Validator
}

func ValidateDecreeContext(dc *DecreeContext) error {
	if dc == nil {
		return fmt.Errorf("decree context is nil")
	}
	if dc.NodeID == "" {
		return fmt.Errorf("empty node ID")
	}
	if dc.Quorum == nil {
		return fmt.Errorf("initial quorum is nil")
	}
	if dc.QuorumHash == "" {
		return fmt.Errorf("initial quorum hash is empty")
	}
	if dc.LM == nil {
		return fmt.Errorf("ledger manager is nil")
	}
	if dc.Validator == nil {
		return fmt.Errorf("validator is nil")
	}
	if dc.StmtChan == nil {
		return fmt.Errorf("statement chan is nil")
	}
	if dc.ExternalizeChan == nil {
		return fmt.Errorf("externalize chan is nil")
	}
	return nil
}

// Decree is an abstractive decision about the consensus value
// that the consensus engine will reach in each round.
type Decree struct {
	index           uint64
	nodeID          string
	quorum          *Quorum
	quorumHash      string
	latestCloseTime uint64

	lm *ledger.Manager

	validator *Validator

	// Previous consensus value.
	prevConsensusValue string

	// States for the nomination protocol.
	votes             mapset.Set
	accepts           mapset.Set
	candidates        mapset.Set
	nominations       map[string]*Statement
	nominationLeaders []string
	nominationRound   int
	nominationStart   bool
	latestNomination  *Nominate
	latestComposite   string // latest composite candidate value

	// States for the ballot protocol.
	currentPhase     BallotPhase
	currentBallot    *Ballot
	pBallot          *Ballot // p
	qBallot          *Ballot // p'
	hBallot          *Ballot // h
	cBallot          *Ballot // c
	ballots          map[string]*Statement
	latestBallotStmt *Statement
	nextValue        string // z
	ballotMsgCount   int

	// Maximum depths of recursion the step function can perform.
	maxRecursions int

	// Channel for broadcasting statements.
	statementChan chan<- *Statement
	// Channel for notifying externalized value.
	externalizeChan chan<- *ExternalizeValue
}

func NewDecree(ctx *DecreeContext) *Decree {
	if err := ValidateDecreeContext(ctx); err != nil {
		log.Fatalf("decree context is invalid: %v", err)
	}

	d := &Decree{
		index:           ctx.Index,
		nodeID:          ctx.NodeID,
		quorum:          ctx.Quorum,
		quorumHash:      ctx.QuorumHash,
		nominationRound: 0,
		nominationStart: false,
		lm:              ctx.LM,
		validator:       ctx.Validator,
		votes:           mapset.NewSet(),
		accepts:         mapset.NewSet(),
		candidates:      mapset.NewSet(),
		nominations:     make(map[string]*Statement),
		currentPhase:    BallotPhasePrepare,
		ballots:         make(map[string]*Statement),
		ballotMsgCount:  0,
		maxRecursions:   10,
		statementChan:   ctx.StmtChan,
		externalizeChan: ctx.ExternalizeChan,
	}

	return d
}

// Nominate nominates a consensus value for the decree.
func (d *Decree) Nominate(prevHash, currHash string) error {
	d.nominationStart = true

	d.nominationRound++

	d.prevConsensusValue = prevHash

	d.updateRoundLeaders()

	// Track whether we have updated any value.
	updated := false
	for _, leader := range d.nominationLeaders {
		if leader == d.nodeID {
			if !d.votes.Contains(currHash) {
				d.votes.Add(currHash)
				updated = true
			}
		} else {
			if stmt, ok := d.nominations[leader]; ok {
				nomination, err := d.getConsensusValue(stmt)
				if err != nil {
					return fmt.Errorf("get new consensus value from statement failed: %v", err)
				}
				if nomination != "" {
					d.votes.Add(nomination)
					updated = true
				}
			}
		}
	}

	if updated {
		if err := d.sendNomination(); err != nil {
			return fmt.Errorf("send nomination failed: %v", err)
		}
	}
	return nil
}

// Recv receives the validated statement and redistributes it to
// the corresponding handler. If it is a nomination statement,
// we send it to the nomination handler. Other types of statements
// belong to ballot protocol, we directly pass the statement to the
// handler can distinguish fine grained type of the statement.
func (d *Decree) Recv(stmt *Statement) error {
	if stmt == nil {
		return errors.New("statement is nil")
	}

	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		err := d.recvNomination(stmt)
		if err != nil {
			return fmt.Errorf("recv nomination failed: %v", err)
		}
	case ultpb.StatementType_PREPARE:
		fallthrough
	case ultpb.StatementType_CONFIRM:
		fallthrough
	case ultpb.StatementType_EXTERNALIZE:
		err := d.recvBallot(stmt)
		if err != nil {
			return fmt.Errorf("recv ballot failed: %v", err)
		}
	}

	return nil
}

// Get the a new consensus value from existing statement.
func (d *Decree) getConsensusValue(stmt *Statement) (string, error) {
	if stmt == nil {
		return "", errors.New("statement is nil")
	}
	maxHashVal, newVote := uint64(0), ""
	nom := stmt.GetNominate()
	for _, vote := range nom.VoteList {
		err := d.validateConsensusValue(vote)
		if err != nil {
			log.Errorf("validate consensus value failed: %v", err)
			continue
		}
		if d.votes.Contains(vote) {
			continue
		}
		hashVal := d.getConsensusValueHash(vote)
		if hashVal > maxHashVal {
			maxHashVal = hashVal
			newVote = vote
		}
	}
	return newVote, nil
}

// Update the leaders of this round of nomination.
func (d *Decree) updateRoundLeaders() {
	quorum := normalizeQuorum(d.quorum, d.nodeID)

	var leaders []string
	leaders = append(leaders, d.nodeID)

	topPriority := d.getNodePriority(quorum, d.nodeID)
	quorumNodes := getQuorumNodes(quorum)
	for _, node := range quorumNodes {
		priority := d.getNodePriority(quorum, node)
		if priority > topPriority {
			topPriority = priority
			leaders = leaders[:0]
		}
		if priority > 0 && priority == topPriority {
			leaders = append(leaders, node)
		}
	}

	// Update the leaders of current round.
	d.nominationLeaders = leaders

	log.Infof("updated round leaders: %v", d.nominationLeaders)
}

// Compute the priority of the node.
func (d *Decree) getNodePriority(quorum *Quorum, nodeID string) uint64 {
	var weight, priority uint64
	if nodeID == d.nodeID {
		weight = math.MaxUint64
	} else {
		weight = d.getNodeWeight(quorum, nodeID)
	}
	if weight > 0 && d.getNodeHash(nodeID, false) <= weight {
		priority = d.getNodeHash(nodeID, true)
		return priority
	}
	return priority
}

// Compute the weight of the node.
func (d *Decree) getNodeWeight(quorum *Quorum, nodeID string) uint64 {
	var weight uint64
	for _, v := range quorum.Validators {
		if v == nodeID {
			weight = uint64(float64(math.MaxInt64) * quorum.Threshold)
			return weight
		}
	}
	for _, q := range quorum.NestQuorums {
		nestWeight := d.getNodeWeight(q, nodeID)
		if nestWeight > 0 {
			weight = uint64(float64(nestWeight) * quorum.Threshold)
			return weight
		}
	}
	return 0
}

// Compute the decree specific hash of the node.
func (d *Decree) getNodeHash(nodeID string, isPriority bool) uint64 {
	hashN, hashP := uint64(1), uint64(2)
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, d.index)
	buf = append(buf, []byte(d.prevConsensusValue)...)
	if isPriority {
		binary.PutUvarint(buf, hashP)
	} else {
		binary.PutUvarint(buf, hashN)
	}
	binary.PutUvarint(buf, uint64(d.nominationRound))
	buf = append(buf, []byte(nodeID)...)
	// Compute the SHA256 hash and use the first 8 bytes as the result.
	result := crypto.SHA256HashUint64(buf)
	return result
}

// Compute the decree specific hash of the consensus value.
func (d *Decree) getConsensusValueHash(vote string) uint64 {
	hashK := uint64(3)
	buf := make([]byte, binary.MaxVarintLen64)
	binary.PutUvarint(buf, d.index)
	buf = append(buf, []byte(d.prevConsensusValue)...)
	binary.PutUvarint(buf, hashK)
	binary.PutUvarint(buf, uint64(d.nominationRound))
	buf = append(buf, []byte(vote)...)
	// Compute the SHA256 hash and use the first 8 bytes as the result.
	result := crypto.SHA256HashUint64(buf)
	return result
}

// Accept statement by checking the following two conditions:
// 1. Voted nodes of the statment form V-blocking for local node.
// 2. All the nodes in the quorum have voted the statement.
func (d *Decree) federatedAccept(voteFilter func(*Statement) bool, acceptFilter func(*Statement) bool, stmts map[string]*Statement) bool {
	// Filter the nodes with voteFilter.
	nodes := mapset.NewSet()
	for n, s := range stmts {
		if voteFilter(s) {
			nodes.Add(n)
		}
	}

	// Check the v-blocking condition.
	if isVblocking(d.quorum, nodes) {
		return true
	}

	// Filter nodes with acceptFilter.
	for n, s := range stmts {
		if acceptFilter(s) {
			nodes.Add(n)
		}
	}

	// Check the quorum condition.
	subnodes := mapset.NewSet()
	for {
		if nodes.Equal(subnodes) {
			break
		}
		subnodes.Clear()
		for n := range nodes.Iter() {
			nodeID := n.(string)
			q := d.getStatementQuorum(stmts[nodeID])
			if q != nil && isQuorumSlice(q, nodes) {
				subnodes.Add(nodeID)
			}
		}
		nodes = nodes.Intersect(subnodes)
	}

	if isQuorumSlice(d.quorum, nodes) {
		return true
	}
	return false
}

// Ratify statement by checking whether all the nodes in the quorum have
// voted the statement. The concrete meaning of this function depends on
// the type of filter supplied. The function will behave like ratification
// when the filter generates voted nodes for a statement and will work as
// confirmation when the filter generates accepted nodes for a statement.
func (d *Decree) federatedRatify(filter func(*Statement) bool, stmts map[string]*Statement) bool {
	// Filter nodes with voteFilter.
	nodes := mapset.NewSet()
	for n, s := range stmts {
		if filter(s) {
			nodes.Add(n)
		}
	}

	// Check the quorum condition.
	subnodes := mapset.NewSet()
	for {
		if nodes.Equal(subnodes) {
			break
		}
		subnodes.Clear()
		for n := range nodes.Iter() {
			nodeID := n.(string)
			q := d.getStatementQuorum(stmts[nodeID])
			if q != nil && isQuorumSlice(q, nodes) {
				subnodes.Add(nodeID)
			}
		}
		nodes = nodes.Intersect(subnodes)
	}

	if isQuorumSlice(d.quorum, nodes) {
		return true
	}
	return false
}

// Extract the quorum from the statement.
func (d *Decree) getStatementQuorum(stmt *Statement) *Quorum {
	var quorum *Quorum
	// Extract the hash of the quorum.
	var hash string
	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		nominate := stmt.GetNominate()
		hash = nominate.QuorumHash
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		hash = prepare.QuorumHash
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		hash = confirm.QuorumHash
	case ultpb.StatementType_EXTERNALIZE:
		quorum = getSingletonQuorum(d.nodeID)
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	if quorum == nil && hash != "" {
		quorum, err := d.validator.GetQuorum(hash)
		if err != nil || quorum == nil {
			// This should not happen as we will only receive
			// validated statements with full information in
			// consensus protocol.
			log.Fatalf("failed to get quorum of hash: %s", hash)
		}
		return quorum
	}

	return quorum
}

// ============  Nomination Protocol ============
// Receive the nomination statement from peers or local node.
func (d *Decree) recvNomination(stmt *Statement) error {
	nom := stmt.GetNominate()

	// Check the validity of votes and accepts.
	if len(nom.VoteList)+len(nom.AcceptList) == 0 {
		return errors.New("vote and accept list is empty")
	}

	log.Infow("recv nomination", "nodeID", stmt.NodeID, "votes", len(nom.VoteList), "accepts", len(nom.AcceptList))

	// Check whether the existing nomination of the remote node
	// is the proper subset of the new nomination and save the
	// new nomination statement.
	if s, ok := d.nominations[stmt.NodeID]; ok {
		if isNewerNomination(s.GetNominate(), nom) {
			d.nominations[stmt.NodeID] = stmt
		}
	} else {
		d.nominations[stmt.NodeID] = stmt
	}

	if d.nominationStart == false {
		return errors.New("nomination has stopped")
	}

	acceptUpdated, candidateUpdated, err := d.promoteVotes(nom)
	if err != nil {
		return fmt.Errorf("promote votes failed: %v", err)
	}

	// Send new nomination if votes changed.
	if acceptUpdated {
		log.Debug("nomination accepted votes changed")
		d.sendNomination()
	}

	// Start the phase of ballot protocol if candidates have changed.
	if candidateUpdated {
		log.Debug("nomination candidate votes changed")
		compValue, err := d.combineCandidates()
		if err != nil {
			return fmt.Errorf("combine candidates failed: %v", err)
		}
		d.latestComposite = compValue
		d.updateBallotPhase(compValue, false)
	}

	return nil
}

// Assemble a nomination and broadcast it to other peers.
func (d *Decree) sendNomination() error {
	// Create an abstract nomination statement.
	nom := &Nominate{
		QuorumHash: d.quorumHash,
	}
	for vote := range d.votes.Iter() {
		nom.VoteList = append(nom.VoteList, vote.(string))
	}
	for accept := range d.accepts.Iter() {
		nom.AcceptList = append(nom.AcceptList, accept.(string))
	}

	stmt := &Statement{
		StatementType: ultpb.StatementType_NOMINATE,
		NodeID:        d.nodeID,
		Index:         d.index,
		Stmt:          &ultpb.Statement_Nominate{Nominate: nom},
	}

	if err := d.recvNomination(stmt); err != nil {
		return fmt.Errorf("receive local nomination failed: %v", err)
	}

	// Broadcast the nomination if it is new.
	if isNewerNomination(d.latestNomination, nom) {
		d.latestNomination = nom
		d.statementChan <- stmt
	}

	return nil
}

// Promote votes to accepts and accepts to candidates for nomination.
func (d *Decree) promoteVotes(newNom *Nominate) (bool, bool, error) {
	acceptUpdated := false
	for _, vote := range newNom.VoteList {
		if d.accepts.Contains(vote) {
			continue
		}
		// Use federated accept to promote values from votes to accepts.
		if !d.federatedAccept(voteFilter(vote), acceptFilter(vote), d.nominations) {
			log.Errorw("nomination federated accept vote failed", "vote", vote)
			continue
		}
		// Validate the vote before updating votes and accepts.
		if err := d.validateConsensusValue(vote); err != nil {
			log.Errorf("nomination validate vote failed: %v", err)
			continue
		}
		d.votes.Add(vote)
		d.accepts.Add(vote)
		acceptUpdated = true
	}

	candidateUpdated := false
	for _, accept := range newNom.AcceptList {
		if d.candidates.Contains(accept) {
			continue
		}
		// Use federated ratify (confirmation) to promote values from accepts to condidates.
		if !d.federatedRatify(acceptFilter(accept), d.nominations) {
			log.Errorw("nomination federated ratify vote failed", "accept", accept)
			continue
		}
		d.candidates.Add(accept)
		candidateUpdated = true
	}

	return acceptUpdated, candidateUpdated, nil
}

// Combine the values in the candidates set into a single candidate value.
func (d *Decree) combineCandidates() (string, error) {
	// Query the lastest closed ledger header.
	headerHash := d.lm.CurrLedgerHeaderHash()

	candidate := &ConsensusValue{}
	var hash string

	var vals []*ConsensusValue
	for cs := range d.candidates.Iter() {
		cand := cs.(string)
		canb, err := b58.Decode(cand)
		if err != nil {
			return "", fmt.Errorf("decode candidate failed: %v", err)
		}

		hash = bytesOr(hash, crypto.SHA256Hash(canb))

		cv, err := ultpb.DecodeConsensusValue(canb)
		if err != nil {
			return "", fmt.Errorf("decode consensus value failed: %v", err)
		}

		// Choose the largest propose time for the combined value.
		if cv.ProposeTime > candidate.ProposeTime {
			candidate.ProposeTime = cv.ProposeTime
		}
		vals = append(vals, cv)
	}

	// Find the txset that contains the largest number of transactions.
	var txset *TxSet
	var txsetHash string
	for _, cv := range vals {
		ts, err := d.validator.GetTxSet(cv.TxSetHash)
		if err != nil || ts == nil {
			continue
		}
		if ts.PrevLedgerHash != headerHash {
			continue
		}
		if txset == nil || compareTxSet(txsetHash, txset, cv.TxSetHash, ts, ledger.GenesisBaseFee, hash) {
			txset = ts
			txsetHash = cv.TxSetHash
		}
	}

	// Deep copy a new txset.
	newTxSet := (pb.Clone(txset)).(*TxSet)

	// TODO(bobonovski) trim invalid tx

	newTxSetHash, err := ultpb.GetTxSetKey(newTxSet)
	if err != nil {
		return "", fmt.Errorf("compute tx set hash failed: %v", err)
	}

	err = d.validator.RecvTxSet(newTxSetHash, newTxSet)
	if err != nil {
		return "", fmt.Errorf("sync quorum to validator failed: %v", err)
	}

	candidate.TxSetHash = newTxSetHash

	candb, err := ultpb.Encode(candidate)
	if err != nil {
		return "", fmt.Errorf("encode combine consensus value failed: %v", err)
	}
	candStr := b58.Encode(candb)

	return candStr, nil
}

// ============  Ballot Protocol ============
// Receive the ballot statement from peer or local node.
func (d *Decree) recvBallot(stmt *Statement) error {
	// Make sure we received statement with the same decree index.
	if stmt.Index != d.index {
		log.Errorf("recv incompatible ballot index: local %d, recv %d", d.index, stmt.Index)
		return errors.New("ballot indices are incompatible")
	}

	log.Infow("recv ballot", "nodeID", stmt.NodeID, "decreeIdx", stmt.Index)

	// Skip the statement if it is old.
	if s, ok := d.ballots[stmt.NodeID]; ok {
		if !isNewerBallot(s, stmt) {
			return nil
		}
	}

	// Check the validity of the ballot.
	if err := d.validateBallot(stmt); err != nil {
		return fmt.Errorf("ballot is invalid: %v", err)
	}

	if d.currentPhase != BallotPhaseExternalize {
		d.ballots[stmt.NodeID] = stmt
		// Try to advance the current ballot state.
		if err := d.step(stmt); err != nil {
			return fmt.Errorf("step forward ballot state failed: %v", err)
		}
	} else {
		wb := getWorkingBallot(stmt)
		if d.cBallot.Value != wb.Value {
			return errors.New("incompatible working ballot value")
		}
		d.ballots[stmt.NodeID] = stmt
	}

	return nil
}

// Assemble a ballot statement based on current ballot phase and broadcast it.
func (d *Decree) sendBallot() error {
	d.checkBallotInvariants()

	stmt := &Statement{
		NodeID: d.nodeID,
		Index:  d.index,
	}
	switch d.currentPhase {
	case BallotPhasePrepare:
		prepare := &Prepare{
			B:          d.currentBallot,
			P:          d.pBallot,
			Q:          d.qBallot,
			QuorumHash: d.quorumHash,
		}
		if d.cBallot != nil {
			prepare.LC = d.cBallot.Counter
		}
		if d.hBallot != nil {
			prepare.HC = d.hBallot.Counter
		}
		stmt.StatementType = ultpb.StatementType_PREPARE
		stmt.Stmt = &ultpb.Statement_Prepare{Prepare: prepare}
	case BallotPhaseConfirm:
		confirm := &Confirm{
			B:          d.currentBallot,
			PC:         d.pBallot.Counter,
			LC:         d.cBallot.Counter,
			HC:         d.hBallot.Counter,
			QuorumHash: d.quorumHash,
		}
		stmt.StatementType = ultpb.StatementType_CONFIRM
		stmt.Stmt = &ultpb.Statement_Confirm{Confirm: confirm}
	case BallotPhaseExternalize:
		ext := &Externalize{
			B:          d.cBallot,
			HC:         d.hBallot.Counter,
			QuorumHash: d.quorumHash,
		}
		stmt.StatementType = ultpb.StatementType_EXTERNALIZE
		stmt.Stmt = &ultpb.Statement_Externalize{Externalize: ext}
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	// Check whether the statement has been processed.
	s, ok := d.ballots[stmt.NodeID]
	if !ok || !pb.Equal(s, stmt) {
		if err := d.recvBallot(stmt); err != nil {
			log.Errorf("recv local ballot failed: %v", err)
			return fmt.Errorf("recv local ballot failed: %v", err)
		}
		if d.currentBallot != nil && (d.latestBallotStmt == nil || isNewerBallot(d.latestBallotStmt, stmt)) {
			d.latestBallotStmt = stmt
			// Broadcast the ballot.
			d.statementChan <- stmt
		}
	}

	return nil
}

// Step the ballot state forward with the input ballot statement.
func (d *Decree) step(stmt *Statement) error {
	d.ballotMsgCount += 1
	if d.ballotMsgCount == d.maxRecursions {
		return errors.New("max number of invoking reached in step")
	}

	log.Debugf("ballot msg count increase to: %d", d.ballotMsgCount)

	// Update the internal ballot state by the following operations:
	// 1. Accept the prepared statement.
	// 2. Confirm the prepared statement.
	// 3. Accept the commit statement.
	// 4. Confirm the commit statement.
	updated := false
	updated = d.acceptPrepared(stmt) || updated
	updated = d.confirmPrepared(stmt) || updated
	updated = d.acceptCommit(stmt) || updated
	updated = d.confirmCommit(stmt) || updated

	if d.ballotMsgCount == 1 {
		counterUpdated := false
		for {
			counterUpdated = d.update()
			updated = counterUpdated || updated
			if counterUpdated == false {
				break
			}
		}
	}

	d.ballotMsgCount -= 1

	log.Debugf("ballot msg count decrease to: %d", d.ballotMsgCount)

	if updated {
		d.statementChan <- d.latestBallotStmt
	}

	return nil
}

// Update the state of the ballot.
func (d *Decree) update() bool {
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	counters := mapset.NewSet()
	for _, s := range d.ballots {
		switch s.StatementType {
		case ultpb.StatementType_PREPARE:
			prepare := s.GetPrepare()
			counters.Add(prepare.B.Counter)
		case ultpb.StatementType_CONFIRM:
			confirm := s.GetConfirm()
			counters.Add(confirm.B.Counter)
		case ultpb.StatementType_EXTERNALIZE:
			counters.Add(math.MaxUint32)
		default:
			log.Fatal(ErrUnknownStmtType)
		}
	}

	target := uint32(0)
	if d.currentBallot != nil {
		target = d.currentBallot.Counter
	}
	counters.Add(target)

	// Sort the counters in ascending order.
	var sortedCounters []uint32
	for c := range counters.Iter() {
		v := c.(uint32)
		sortedCounters = append(sortedCounters, v)
	}
	sort.SliceStable(sortedCounters, func(i, j int) bool {
		return sortedCounters[i] < sortedCounters[j]
	})

	// Statement filter.
	stmtFilter := func(counter uint32) func(*Statement) bool {
		return func(stmt *Statement) bool {
			var cond bool
			switch stmt.StatementType {
			case ultpb.StatementType_PREPARE:
				prepare := stmt.GetPrepare()
				cond = counter < prepare.B.Counter
			case ultpb.StatementType_CONFIRM:
				confirm := stmt.GetConfirm()
				cond = counter < confirm.B.Counter
			default:
				cond = counter != math.MaxUint32
			}
			return cond
		}
	}

	// Find the smallest compatible counter.
	nodes := mapset.NewSet()
	for _, c := range sortedCounters {
		if c < target {
			break
		}

		filter := stmtFilter(c)
		for _, stmt := range d.ballots {
			if filter(stmt) {
				nodes.Add(stmt.NodeID)
			}
		}

		// Check whether the nodes form v-blocking.
		vb := isVblocking(d.quorum, nodes)
		if vb {
			continue
		}
		if c == target {
			break
		}
		nodes.Clear()

		updated := d.abandonBallot(c)
		return updated
	}

	return false
}

func (d *Decree) abandonBallot(c uint32) bool {
	cv := d.latestComposite
	updated := false

	if cv == "" {
		if d.currentBallot != nil {
			cv = d.currentBallot.Value
		}
	}
	if cv != "" {
		if c == 0 {
			updated = d.updateBallotPhase(cv, true)
		} else {
			updated = d.updateBallotPhaseWithCounter(cv, c)
		}
	}

	return updated
}

// Accept the new ballot statement as prepared using federated voting.
func (d *Decree) acceptPrepared(stmt *Statement) bool {
	// It is only necessary to call this method when
	// current phase is in prepare or confirm.
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	candidates := d.getPreparedCandidates(stmt)

	log.Debugf("get %d candidates for accepting prepared", len(candidates))

	for _, cand := range candidates {
		if d.currentPhase == BallotPhaseConfirm {
			// Skip old or incompatible ballots.
			if !lessAndCompatibleBallots(d.pBallot, cand) {
				continue
			}
			// We should not have accepted prepared ballot that
			// is incompatible with current commit ballot.
			if !compatibleBallots(d.cBallot, cand) {
				log.Fatal("candidate ballot and commit ballot are not compatible")
			}
		}

		// Skip the prepared ballot.
		if d.qBallot != nil && compareBallots(cand, d.qBallot) <= 0 {
			continue
		}
		if d.pBallot != nil && lessAndCompatibleBallots(cand, d.pBallot) {
			continue
		}

		accepted := d.federatedAccept(prepareVoteFilter(cand), prepareAcceptFilter(cand), d.ballots)
		if accepted {
			// Update the prepared ballot.
			updated := d.setAcceptPrepared(cand)
			if d.cBallot != nil && d.hBallot != nil {
				if (d.pBallot != nil && lessAndIncompatibleBallots(d.hBallot, d.pBallot)) ||
					(d.qBallot != nil && lessAndIncompatibleBallots(d.hBallot, d.qBallot)) {
					if d.currentPhase == BallotPhasePrepare {
						d.cBallot.Reset()
						updated = true
					}
				}
			}
			if updated {
				log.Debug("accept prepared ballot updated")
				d.sendBallot()
			}
			return updated
		}
	}

	return false
}

// Update internal q and p ballot states with the new accepted ballot.
func (d *Decree) setAcceptPrepared(b *Ballot) bool {
	updated := false
	if d.pBallot != nil { // b < p
		cmp := compareBallots(b, d.pBallot)
		if cmp < 0 {
			// Check whether we can update q ballot.
			if d.qBallot == nil || (compareBallots(b, d.qBallot) > 0 && !compatibleBallots(b, d.pBallot)) {
				d.qBallot = b
				updated = true
			}
		} else if cmp > 0 { // q < p < b
			if !compatibleBallots(b, d.pBallot) {
				d.qBallot = d.pBallot
			}
			d.pBallot = b
			updated = true
		}
	} else {
		d.pBallot = b
		updated = true
	}
	return updated
}

// Extract unique prepared candidate ballots from statement.
func (d *Decree) getPreparedCandidates(stmt *Statement) []*Ballot {
	// Filter duplicate ballots that have the same value.
	ballots := mapset.NewSet()

	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		ballots.Add(*prepare.B)
		if prepare.P != nil {
			ballots.Add(*prepare.P)
		}
		if prepare.Q != nil {
			ballots.Add(*prepare.Q)
		}
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		ballots.Add(Ballot{Value: confirm.B.Value, Counter: confirm.PC})
		ballots.Add(Ballot{Value: confirm.B.Value, Counter: math.MaxUint32})
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		ballots.Add(Ballot{Value: ext.B.Value, Counter: math.MaxUint32})
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	var candidates []*Ballot
	if ballots.Cardinality() == 0 {
		return candidates
	}

	// Process ballots in descending order.
	candSet := mapset.NewSet()
	for ballot := range ballots.Iter() {
		b := ballot.(Ballot)
		for _, stmt := range d.ballots {
			switch stmt.StatementType {
			case ultpb.StatementType_PREPARE:
				prepare := stmt.GetPrepare()
				if lessAndCompatibleBallots(prepare.B, &b) {
					candSet.Add(*prepare.B)
				}
				if prepare.P != nil && lessAndCompatibleBallots(prepare.P, &b) {
					candSet.Add(*prepare.P)
				}
				if prepare.Q != nil && lessAndCompatibleBallots(prepare.Q, &b) {
					candSet.Add(*prepare.Q)
				}
			case ultpb.StatementType_CONFIRM:
				confirm := stmt.GetConfirm()
				if compatibleBallots(confirm.B, &b) {
					candSet.Add(b)
					// Check the highest accepted prepared ballot counter.
					if confirm.PC < b.Counter {
						candSet.Add(Ballot{Value: b.Value, Counter: confirm.PC})
					}
				}
			case ultpb.StatementType_EXTERNALIZE:
				ext := stmt.GetExternalize()
				if compatibleBallots(ext.B, &b) {
					candSet.Add(b)
				}
			default:
				log.Fatal(ErrUnknownStmtType)
			}
		}
	}

	for v := range candSet.Iter() {
		b := v.(Ballot)
		candidates = append(candidates, &b)
	}

	// Sort the candidates in descending order.
	sort.Sort(BallotSlice(candidates))

	return candidates
}

// Confirm new ballot statement as prepared by ratifying accepted prepared ballots.
func (d *Decree) confirmPrepared(stmt *Statement) bool {
	if d.currentPhase != BallotPhasePrepare || d.pBallot == nil {
		return false
	}

	candidates := d.getPreparedCandidates(stmt)

	log.Debugf("get %d candidates for confirming prepared", len(candidates))

	// Candidate for the higher ballot.
	var hCandidate *ultpb.Ballot

	i := 0
	for ; i < len(candidates); i++ {
		cand := candidates[i]
		// Skip the ballot if it is impossible to find
		// a higher ballot that is confirmed prepared.
		if d.hBallot != nil && compareBallots(cand, d.hBallot) <= 0 {
			break
		}

		if d.federatedRatify(prepareAcceptFilter(cand), d.ballots) {
			hCandidate = cand
			break
		}
	}

	if hCandidate != nil {
		// Candidate for commit ballot.
		var cCandidate *Ballot
		curb := d.currentBallot
		if curb == nil {
			curb = &Ballot{}
		}
		// Find a new commit ballot.
		if d.cBallot == nil &&
			(d.pBallot == nil || !lessAndIncompatibleBallots(hCandidate, d.pBallot)) &&
			(d.qBallot == nil || !lessAndIncompatibleBallots(hCandidate, d.qBallot)) {
			for ; i < len(candidates); i++ {
				cand := candidates[i]
				if compareBallots(cand, curb) < 0 {
					break
				}
				if !lessAndCompatibleBallots(cand, hCandidate) {
					continue
				}
				if d.federatedRatify(prepareAcceptFilter(cand), d.ballots) {
					cCandidate = cand
					continue
				}
				// Candidates are sorted in descending order,
				// if we cannot ratify the current candidate we
				// can skip the rest of the checks.
				break
			}
		}
		updated := d.setConfirmPrepared(cCandidate, hCandidate)
		return updated
	}

	return false
}

// Update internal c and h ballots with new confirmed prepared ballots.
func (d *Decree) setConfirmPrepared(cb *Ballot, hb *Ballot) bool {
	// Save the next ballot value to use.
	d.nextValue = hb.Value

	updated := false
	if d.currentBallot == nil || compatibleBallots(d.currentBallot, hb) {
		if d.hBallot == nil || compareBallots(d.hBallot, hb) < 0 {
			d.hBallot = hb
			updated = true
		}
		if d.cBallot == nil || compareBallots(cb, d.cBallot) < 0 {
			d.cBallot = cb
			updated = true
		}
	}

	updated = d.updateBallotIfNeeded(hb) || updated
	if updated {
		d.sendBallot()
	}

	return updated
}

// Accept commit statement for the ballot.
func (d *Decree) acceptCommit(stmt *Statement) bool {
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	// Create a commit ballot from the input statement.
	ballot := &Ballot{}
	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		if prepare.LC == 0 { // No commit ballot yet.
			return false
		}
		ballot.Value = prepare.B.Value
		ballot.Counter = prepare.HC
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		ballot.Value = confirm.B.Value
		ballot.Counter = confirm.HC
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		ballot.Value = ext.B.Value
		ballot.Counter = ext.HC
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	// Make sure the new ballot is compatible with local higher ballot.
	if d.currentPhase == BallotPhaseConfirm && !compatibleBallots(ballot, d.hBallot) {
		return false
	}

	// Collect all the candidate counters.
	counters := d.getCommitCounters(ballot)

	filter := func(l, r uint32) bool {
		return d.federatedAccept(commitVoteFilter(ballot, l, r), commitAcceptFilter(ballot, l, r), d.ballots)
	}

	lb, rb := d.findCommitInterval(counters, filter)

	log.Debugf("get commit interval %d,%d for accepting commit", lb, rb)

	if lb != 0 {
		if d.currentPhase != BallotPhaseConfirm || rb > d.hBallot.Counter {
			cBallot := &Ballot{Value: ballot.Value, Counter: lb}
			hBallot := &Ballot{Value: ballot.Value, Counter: rb}
			updated := d.setAcceptCommit(cBallot, hBallot)
			return updated
		}
	}

	return false
}

// Update internal c and h ballots with new accepted commit ballots.
func (d *Decree) setAcceptCommit(cb *Ballot, hb *Ballot) bool {
	// Save the next ballot value to use.
	d.nextValue = hb.Value

	updated := false
	if d.hBallot == nil || d.cBallot == nil || compareBallots(d.hBallot, hb) != 0 || compareBallots(d.cBallot, cb) != 0 {
		d.hBallot = hb
		d.cBallot = cb
		updated = true
	}

	// TODO(bobonovski) rethinking the logic
	if d.currentPhase == BallotPhasePrepare {
		d.currentPhase = BallotPhaseConfirm
		if d.currentBallot != nil && !lessAndCompatibleBallots(hb, d.currentBallot) {
			d.updateBallot(hb)
		}
		if d.qBallot != nil {
			d.qBallot.Reset()
		}
		updated = true
	}

	if updated {
		d.updateBallotIfNeeded(d.hBallot)
		d.sendBallot()
	}

	return updated
}

// Find lower and higher counter for commit ballot.
func (d *Decree) findCommitInterval(counters []uint32, filter func(l, r uint32) bool) (uint32, uint32) {
	var lb, rb uint32

	for _, c := range counters {
		l, r := uint32(0), uint32(0)
		if lb == 0 {
			l, r = c, c
		} else {
			l, r = c, rb
		}

		if filter(l, r) {
			lb, rb = l, r
		} else if lb != 0 {
			break
		}
	}

	return lb, rb
}

// Extract commit lower and higher counters.
func (d *Decree) getCommitCounters(b *Ballot) []uint32 {
	counters := mapset.NewSet()

	for _, s := range d.ballots {
		switch s.StatementType {
		case ultpb.StatementType_PREPARE:
			prepare := s.GetPrepare()
			if compatibleBallots(b, prepare.B) {
				if prepare.LC > 0 {
					counters.Add(prepare.LC)
					counters.Add(prepare.HC)
				}
			}
		case ultpb.StatementType_CONFIRM:
			confirm := s.GetConfirm()
			if compatibleBallots(b, confirm.B) {
				counters.Add(confirm.LC)
				counters.Add(confirm.HC)
			}
		case ultpb.StatementType_EXTERNALIZE:
			ext := s.GetExternalize()
			if compatibleBallots(b, ext.B) {
				counters.Add(ext.B.Counter)
				counters.Add(ext.HC)
				counters.Add(math.MaxUint32)
			}
		default:
			log.Fatal(ErrUnknownStmtType)
		}
	}

	var ctrs []uint32
	for c := range counters.Iter() {
		counter := c.(uint32)
		ctrs = append(ctrs, counter)
	}
	sort.SliceStable(ctrs, func(i, j int) bool {
		return ctrs[i] > ctrs[j]
	})

	return ctrs
}

// Confirm the commit ballot using federated voting.
func (d *Decree) confirmCommit(stmt *Statement) bool {
	if d.currentPhase != BallotPhaseConfirm {
		return false
	}

	if d.hBallot == nil || d.cBallot == nil {
		return false
	}

	ballot := &Ballot{}
	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		return false
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		ballot.Value = confirm.B.Value
		ballot.Counter = confirm.HC
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		ballot.Value = ext.B.Value
		ballot.Counter = ext.HC
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	if !compatibleBallots(d.cBallot, ballot) {
		return false
	}

	counters := d.getCommitCounters(ballot)

	filter := func(l, r uint32) bool {
		return d.federatedRatify(commitAcceptFilter(ballot, l, r), d.ballots)
	}

	lb, rb := d.findCommitInterval(counters, filter)

	log.Debugf("get commit interval %d,%d for confirming commit", lb, rb)

	if lb != 0 {
		cBallot := &Ballot{Value: ballot.Value, Counter: lb}
		hBallot := &Ballot{Value: ballot.Value, Counter: rb}
		updated := d.setConfirmCommit(cBallot, hBallot)
		return updated
	}

	return false
}

// Update internal c and h ballots with confirmed commit ballot
// and trigger externalization of the commit value.
func (d *Decree) setConfirmCommit(cb *Ballot, hb *Ballot) bool {
	// Update the commit and higher ballots.
	d.cBallot = cb
	d.hBallot = hb

	d.updateBallotIfNeeded(d.hBallot)

	// Change phase to externalize.
	d.currentPhase = BallotPhaseExternalize

	d.sendBallot()

	// Stop nomination protocol.
	d.nominationStart = false

	// Trigger externalization.
	d.externalizeChan <- &ExternalizeValue{
		Index: d.index,
		Value: d.cBallot.Value,
	}

	return true
}

// Update the current ballot if needed.
func (d *Decree) updateBallotIfNeeded(b *Ballot) bool {
	updated := false
	if d.currentBallot == nil || compareBallots(d.currentBallot, b) < 0 {
		d.updateBallot(b)
		updated = true
	}
	return updated
}

// Update the current ballot phase.
func (d *Decree) updateBallotPhase(val string, force bool) bool {
	if !force && d.currentBallot != nil {
		return false
	}

	counter := uint32(1)
	if d.currentBallot != nil {
		counter = d.currentBallot.Counter + 1
	}

	updated := d.updateBallotPhaseWithCounter(val, counter)

	return updated
}

// Update the current ballot phase with specified counter.
func (d *Decree) updateBallotPhaseWithCounter(val string, counter uint32) bool {
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	b := &Ballot{Counter: counter, Value: val}
	if d.nextValue != "" {
		b.Value = d.nextValue
	}

	updated := d.updateBallotValue(b)
	if updated {
		log.Debug("current ballot updated")
		d.sendBallot()
	}

	return updated
}

// Update the current ballot value.
func (d *Decree) updateBallotValue(b *Ballot) bool {
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	updated := false

	if d.currentBallot == nil {
		d.updateBallot(b)
		updated = true
	} else {
		if d.cBallot != nil && !compatibleBallots(d.cBallot, b) {
			return false
		}

		if compareBallots(d.currentBallot, b) <= 0 {
			d.updateBallot(b)
			updated = true
		}
	}

	d.checkBallotInvariants()

	return updated
}

// Update the current ballot.
func (d *Decree) updateBallot(b *Ballot) {
	if d.currentPhase == BallotPhaseExternalize {
		log.Fatal("should not update ballot in externalize phase")
	}

	if d.currentBallot != nil && compareBallots(d.currentBallot, b) > 0 {
		log.Fatal("cannot update current ballot with smaller one")
	}

	// Deep copy the ballot.
	d.currentBallot = pb.Clone(b).(*Ballot)

	if d.hBallot != nil && !compatibleBallots(d.currentBallot, d.hBallot) {
		d.hBallot.Reset()
	}
}

// Check invariants of ballot states. Any detected errors could
// imply some bugs in the consensus protocol.
func (d *Decree) checkBallotInvariants() {
	if d.currentBallot != nil && d.currentBallot.Counter == 0 {
		log.Fatal("current ballot is not nil but counter is zero")
	}

	// q and p ballots are the two highest ballots accepted as prepared
	// such that q is incompatible with p.
	if d.pBallot != nil && d.qBallot != nil {
		cond := compareBallots(d.qBallot, d.pBallot) <= 0 && !compatibleBallots(d.qBallot, d.pBallot)
		if !cond {
			log.Fatal("q ballot and p ballot invariant not satisfied")
		}
	}

	// cBallot <= hBallot <= currentBallot
	if d.hBallot != nil {
		if d.currentBallot == nil {
			log.Fatal("high ballot is not nil but current ballot is nil")
		}
		cond := compareBallots(d.hBallot, d.currentBallot) <= 0 && compatibleBallots(d.hBallot, d.currentBallot)
		if !cond {
			log.Fatal("current ballot and higher ballot invariant not satisfied")
		}
	}

	if d.cBallot != nil {
		if d.currentBallot == nil {
			log.Fatal("commit ballot is not nil but current ballot is nil")
		}
		cond := compareBallots(d.cBallot, d.hBallot) <= 0 && compatibleBallots(d.cBallot, d.hBallot)
		if !cond {
			log.Fatal("commit ballot and higher ballot invariant not satisfied")
		}
		cond = compareBallots(d.hBallot, d.currentBallot) <= 0 && compatibleBallots(d.hBallot, d.currentBallot)
		if !cond {
			log.Fatal("current ballot and higher ballot invariant not satisfied")
		}
	}

	switch d.currentPhase {
	case BallotPhasePrepare:
	case BallotPhaseConfirm:
		if d.cBallot == nil {
			log.Fatal("commit ballot should not be nil in confirm phase")
		}
	case BallotPhaseExternalize:
		if d.cBallot == nil {
			log.Fatal("commit ballot should not be nil in externalize phase")
		}
		if d.hBallot == nil {
			log.Fatal("higher ballot should not be nil in externalize phase")
		}
	default:
		log.Fatal(ErrUnknownStmtType)
	}
}

// Validate ballot by checking:
// 1. Ballot counters are in expected states.
// 2. Ballot values are normal and satisfy consensus constraints.
func (d *Decree) validateBallot(stmt *Statement) error {
	if stmt == nil {
		return fmt.Errorf("ballot statement is nil")
	}

	fromSelf := d.nodeID == stmt.NodeID

	// Value set for checking validity of ballot value.
	values := mapset.NewSet()

	// Validate quorum from the statement.
	quorum := d.getStatementQuorum(stmt)
	if err := ValidateQuorum(quorum, 0, false); err != nil {
		return fmt.Errorf("validate quorum failed: %v", err)
	}

	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		// Checking counter sanity.
		if !fromSelf && prepare.B.Counter == 0 {
			return errors.New("prepare ballot counter is zero and not self msg")
		}
		cond := compareBallots(prepare.Q, prepare.P) <= 0 && !compatibleBallots(prepare.Q, prepare.P)
		if prepare.Q != nil && prepare.P != nil && !cond {
			return errors.New("prepare q ballot and p ballot are not in expected states")
		}
		cond = prepare.HC == 0 || (prepare.P != nil && prepare.HC <= prepare.P.Counter)
		if !cond {
			return errors.New("prepare p ballot and higher counters are not in expected states")
		}
		cond = prepare.LC == 0 || (prepare.HC != 0 && prepare.HC <= prepare.B.Counter && prepare.LC <= prepare.HC)
		if !cond {
			return errors.New("prepare ballot counters are not in expected states")
		}
		if prepare.B.Counter > 0 {
			values.Add(prepare.B.Value)
		}
		if prepare.P != nil {
			values.Add(prepare.P.Value)
		}
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		// Check counter sanity.
		if confirm.B.Counter == 0 {
			return fmt.Errorf("confirm current ballot counter should not be zero")
		}
		cond := confirm.LC <= confirm.HC && confirm.HC <= confirm.B.Counter
		if !cond {
			return fmt.Errorf("confirm ballot counters are not in expected states")
		}
		values.Add(confirm.B.Value)
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		// Check counter sanity.
		cond := ext.B.Counter > 0 && ext.B.Counter <= ext.HC
		if !cond {
			return fmt.Errorf("externalize ballot counters are not in expected states")
		}
		values.Add(ext.B.Value)
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	// Check values against consensus constraints.
	var valueErr error
	for v := range values.Iter() {
		if err := d.validateConsensusValue(v.(string)); err != nil {
			valueErr = err
		}
	}

	return valueErr
}

// Validate consensus value.
func (d *Decree) validateConsensusValue(val string) error {
	vb, err := b58.Decode(val)
	if err != nil {
		return fmt.Errorf("decode hex string failed: %v", err)
	}

	cv, err := ultpb.DecodeConsensusValue(vb)
	if err != nil {
		return fmt.Errorf("decode consensus value failed: %v", err)
	}

	err = d.validateConsensusValueFull(cv, false)
	if err != nil {
		return fmt.Errorf("consensus value is not valid: %v", err)
	}
	return nil
}

// Validate consensus value with full checks.
func (d *Decree) validateConsensusValueFull(cv *ConsensusValue, isNomination bool) error {
	// Check whether the decree index is up to date.
	if !d.lm.LedgerSynced() || d.lm.NextLedgerHeaderSeq() != d.index {
		return errors.New("consensus value is not compatible with ledger state")
	}
	// The propose time should be larger than the close time in closed ledger.
	header := d.lm.CurrLedgerHeader()
	if header.ConsensusValue != "" {
		vb, err := b58.Decode(header.ConsensusValue)
		if err != nil {
			return fmt.Errorf("decode hex string failed: %v", err)
		}
		headerCV, err := ultpb.DecodeConsensusValue(vb)
		if err != nil {
			return fmt.Errorf("decode ledger header consensus value failed: %v", err)
		}
		if headerCV.CloseTime > cv.ProposeTime {
			return errors.New("consensus value is too old")
		}
	}
	// Check the existance of the txset.
	txset, err := d.lm.GetTxSet(cv.TxSetHash)
	if err != nil || txset == nil {
		return fmt.Errorf("get txset failed: %v", err)
	}
	return nil
}
