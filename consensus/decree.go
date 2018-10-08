package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

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

// Information about externalized value
type ExternalizeValue struct {
	Index uint64 // index of decree
	Value string // consensus value
}

// DecreeContext contains contextual information Decree needs
type DecreeContext struct {
	Index           uint64  // decree index
	NodeID          string  // local node ID
	Quorum          *Quorum // local node quorum
	QuorumHash      string  // local node quorum hash
	LM              *ledger.Manager
	Validator       *Validator
	StmtChan        chan<- *Statement        // channel for broadcasting statement
	ExternalizeChan chan<- *ExternalizeValue // channel for notifying externlized value
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

// Decree is an abstractive decision the consensus engine
// should reach in each round
type Decree struct {
	index           uint64
	nodeID          string
	quorum          *Quorum
	quorumHash      string
	latestCloseTime uint64

	// ledger manager
	lm *ledger.Manager
	// validator
	validator *Validator

	// for nomination protocol
	votes            mapset.Set
	accepts          mapset.Set
	candidates       mapset.Set
	nominations      map[string]*Statement
	nominationRound  int
	nominationStart  bool
	latestNomination *Nominate
	latestComposite  string // latest composite candidate value

	// for ballot protocol
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

	// channel for sending statements
	statementChan chan<- *Statement
	// channel for sending externalized value
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
		statementChan:   ctx.StmtChan,
		externalizeChan: ctx.ExternalizeChan,
	}

	return d
}

// Nominate nominates a consensus value for the decree
func (d *Decree) Nominate(prevHash, currHash string) error {
	d.nominationStart = true

	d.nominationRound++
	// TODO(bobonovski) compute leader weights
	d.votes.Add(currHash) // For test

	if err := d.sendNomination(); err != nil {
		return fmt.Errorf("send nomination failed: %v", err)
	}
	return nil
}

// Recv receives validated statement and redistributes it to
// corresponding route handler. If the statement is a nomination,
// we give it to seperated nomination handler. Other statement types
// belong to ballot protocol, we directly pass the statement to the
// handler which contains its own logic to distinguish fine grained
// statement types.
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

// Accept statement by checking the following two conditions:
// 1. voted nodes of the statment form V-blocking for local node
// 2. all the nodes in the quorum have voted the statement
func (d *Decree) federatedAccept(voteFilter func(*Statement) bool, acceptFilter func(*Statement) bool, stmts map[string]*Statement) bool {
	// filter nodes with voteFilter
	nodes := mapset.NewSet()
	for n, s := range stmts {
		if voteFilter(s) {
			nodes.Add(n)
		}
	}

	// check V-blocking condition
	if isVblocking(d.quorum, nodes) {
		return true
	}

	// filter nodes with acceptFilter
	for n, s := range stmts {
		if acceptFilter(s) {
			nodes.Add(n)
		}
	}

	// check quorum condition
	subnodes := mapset.NewSet()
	for {
		if nodes.Equal(subnodes) {
			break
		}
		for n := range nodes.Iter() {
			nodeID := n.(string)
			q := d.getStatementQuorum(stmts[nodeID])
			if q != nil && isQuorumSlice(q, nodes) {
				subnodes.Add(nodeID)
			}
		}
		nodes = nodes.Intersect(subnodes)
		subnodes.Clear()
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
	// filter nodes with voteFilter
	nodes := mapset.NewSet()
	for n, s := range stmts {
		if filter(s) {
			nodes.Add(n)
		}
	}

	// check quorum condition
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

// Get the quorum from statement
func (d *Decree) getStatementQuorum(stmt *Statement) *Quorum {
	var quorum *Quorum
	// extract quorum hash
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
		quorum, ok := d.validator.GetQuorum(hash)
		if !ok {
			// this should not happen as we will only receive
			// validated statements with full information in
			// consensus protocol
			log.Fatalf("failed to get quorum of hash: %s", hash)
		}
		return quorum
	}

	return quorum
}

/* Nomination Protocol */
// Receive nomination from peers or local node
func (d *Decree) recvNomination(stmt *Statement) error {
	nom := stmt.GetNominate()

	// check validity of votes and accepts
	if len(nom.VoteList)+len(nom.AcceptList) == 0 {
		return errors.New("vote and accept list is empty")
	}

	log.Infow("received nomination", "nodeID", stmt.NodeID, "votes", len(nom.VoteList), "accepts", len(nom.AcceptList))

	// check whether the existing nomination of the remote node
	// is the proper subset of the new nomination and save the
	// new nomination statement
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

	// send new nomination if votes changed
	if acceptUpdated {
		log.Info("nomination accepted votes changed")
		d.sendNomination()
	}

	// start balloting if candidates changed
	if candidateUpdated {
		log.Info("nomination candidate votes changed")
		compValue, err := d.combineCandidates()
		if err != nil {
			return fmt.Errorf("combine candidates failed: %v", err)
		}
		d.latestComposite = compValue
		d.updateBallotPhase(compValue, false)
	}

	return nil
}

// assemble a nomination and broadcast it to other peers
func (d *Decree) sendNomination() error {
	// create an abstract nomination statement
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

	// broadcast the nomination if it is a new one
	if isNewerNomination(d.latestNomination, nom) {
		d.latestNomination = nom
		d.statementChan <- stmt
	}

	return nil
}

// Promote votes to accepts and accepts to candidates for nomination
func (d *Decree) promoteVotes(newNom *Nominate) (bool, bool, error) {
	acceptUpdated := false
	for _, vote := range newNom.VoteList {
		if d.accepts.Contains(vote) {
			continue
		}
		// use federated accept to promote values from votes to accepts
		if !d.federatedAccept(voteFilter(vote), acceptFilter(vote), d.nominations) {
			log.Errorw("nomination federated accept vote failed", "vote", vote)
			continue
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
		// use federated ratify to promote values from accepts to
		// condidates, i.e. confirmation
		if !d.federatedRatify(acceptFilter(accept), d.nominations) {
			log.Errorw("nomination federated ratify vote failed", "accept", accept)
			continue
		}
		d.candidates.Add(accept)
		candidateUpdated = true
	}

	return acceptUpdated, candidateUpdated, nil
}

// Combine the candidates set into a single candidate value
func (d *Decree) combineCandidates() (string, error) {
	// query lastest closed ledger header
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

		// accumulate hash
		hash = bytesOr(hash, crypto.SHA256Hash(canb))

		cv, err := ultpb.DecodeConsensusValue(canb)
		if err != nil {
			return "", fmt.Errorf("decode consensus value failed: %v", err)
		}
		if cv.ProposeTime > candidate.ProposeTime {
			candidate.ProposeTime = cv.ProposeTime
		}

		vals = append(vals, cv)
	}

	// find the txset contains the largest number of tx
	var txset *TxSet
	var txsetHash string
	for _, cv := range vals {
		ts, ok := d.validator.GetTxSet(cv.TxSetHash)
		if !ok {
			continue
		}
		if ts.PrevLedgerHash != headerHash {
			continue
		}
		if txset == nil || len(ts.TxList) > len(txset.TxList) ||
			((len(ts.TxList) == len(txset.TxList)) && lessBytesOr(txsetHash, cv.TxSetHash, hash)) {
			txset = ts
			txsetHash = cv.TxSetHash
		}
	}

	// deep copy a new txset
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

// Check whether the first input string is smaller than the second one
// after byte-wise OR
func lessBytesOr(l string, r string, h string) bool {
	lb, _ := b58.Decode(l)
	rb, _ := b58.Decode(r)
	hb, _ := b58.Decode(h)

	lbuf := bytes.NewBuffer(nil)
	rbuf := bytes.NewBuffer(nil)
	for i, _ := range hb {
		lbuf.WriteByte(lb[i] ^ hb[i])
		rbuf.WriteByte(rb[i] ^ hb[i])
	}

	return lbuf.String() < rbuf.String()
}

// Compute the byte-wise OR bit operation between input bytes
func bytesOr(l string, r string) string {
	lb, _ := b58.Decode(l)
	rb, _ := b58.Decode(r)

	buf := bytes.NewBuffer(nil)
	for i, _ := range lb {
		buf.WriteByte(lb[i] ^ rb[i])
	}

	return buf.String()
}

/* Ballot Protocol */
// Receive ballot statement from peer or local nodes
func (d *Decree) recvBallot(stmt *Statement) error {
	// make sure we received statement with the same decree index
	if stmt.Index != d.index {
		log.Errorf("received incompatible ballot index: local %d, recv %d", d.index, stmt.Index)
	}

	log.Infow("received ballot", "nodeID", stmt.NodeID, "decreeIdx", stmt.Index)

	// skip outdated statement without returning error
	if s, ok := d.ballots[stmt.NodeID]; ok {
		if !isNewerBallot(s, stmt) {
			return nil
		}
	}

	// make sure the ballot is valid
	if err := d.validateBallot(stmt); err != nil {
		return fmt.Errorf("ballot is invalid: %v", err)
	}

	if d.currentPhase != BallotPhaseExternalize {
		d.ballots[stmt.NodeID] = stmt
		// try to advance current ballot state
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

// Assemble a ballot statement based on current ballot phase and broadcast it
func (d *Decree) sendBallot() error {
	d.checkBallotInvariants()

	// create statement
	stmt := &Statement{
		NodeID: d.nodeID,
		Index:  d.index,
	}

	// assemble ballot statement based on current phase
	switch d.currentPhase {
	case BallotPhasePrepare: // Prepare statement
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
	case BallotPhaseConfirm: // Confirm statement
		confirm := &Confirm{
			B:          d.currentBallot,
			PC:         d.pBallot.Counter,
			LC:         d.cBallot.Counter,
			HC:         d.hBallot.Counter,
			QuorumHash: d.quorumHash,
		}
		stmt.StatementType = ultpb.StatementType_CONFIRM
		stmt.Stmt = &ultpb.Statement_Confirm{Confirm: confirm}
	case BallotPhaseExternalize: // Externalize statement
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

	// check whether the statement is already processed
	s, ok := d.ballots[stmt.NodeID]
	if !ok || !pb.Equal(s, stmt) {
		if err := d.recvBallot(stmt); err != nil {
			return fmt.Errorf("recv local ballot failed: %v", err)
		}
		if d.currentBallot != nil && (d.latestBallotStmt == nil || isNewerBallot(d.latestBallotStmt, stmt)) {
			d.latestBallotStmt = stmt
			// broadcast the ballot
			d.statementChan <- stmt
		}
	}

	return nil
}

// Step the ballot state forward with the input ballot statement
func (d *Decree) step(stmt *Statement) error {
	d.ballotMsgCount += 1
	if d.ballotMsgCount == 10 { // TODO(bobonovski) adaptive threshold?
		return errors.New("max number of invoking reached in step")
	}
	// try to update internal ballot states bying following operations:
	// 1. accept the prepared statement
	// 2. confirm the prepared statement
	// 3. accept the commit statement
	// 4. confirm the commit statement
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

	if d.ballotMsgCount == 0 && d.latestBallotStmt != nil {
		d.statementChan <- d.latestBallotStmt
	}

	return nil
}

// Forward ballot counter
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

	// sort the counters in descending order
	var sortedCounters []uint32
	for c := range counters.Iter() {
		v := c.(uint32)
		sortedCounters = append(sortedCounters, v)
	}
	sort.SliceStable(sortedCounters, func(i, j int) bool {
		return sortedCounters[i] > sortedCounters[j]
	})

	// statement filter
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

	// filter statements to get the nodeIDs
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

		// check whether the nodes form v-blocking
		vb := isVblocking(d.quorum, nodes)
		if vb {
			continue
		}
		if c == target {
			break
		}
		d.abandonBallot(c)

		nodes.Clear()
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
	} else {
		if c == 0 {
			updated = d.updateBallotPhase(cv, true)
		} else {
			updated = d.updateBallotPhaseWithCounter(cv, c)
		}
	}

	return updated
}

// Accept new ballot statement as prepared using federated voting
func (d *Decree) acceptPrepared(stmt *Statement) bool {
	// it is only necessary to call this method when
	// current phase is in prepare or confirm.
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	candidates := d.preparedCandidates(stmt)

	log.Infof("get %d candidates for accepting prepared", len(candidates))

	for _, cand := range candidates {
		if d.currentPhase == BallotPhaseConfirm {
			// skip old or incompatible ballots
			if !lessAndCompatibleBallots(d.pBallot, cand) {
				continue
			}
			// we should not have accepted prepared ballot that
			// is incompatible with current commit ballot
			if !compatibleBallots(d.cBallot, cand) {
				log.Fatal("candidate ballot and commit ballot are not compatible")
			}
		}

		// skip prepared ballot
		if d.qBallot != nil && compareBallots(cand, d.qBallot) <= 0 {
			continue
		}
		if d.pBallot != nil && lessAndCompatibleBallots(cand, d.pBallot) {
			continue
		}

		accepted := d.federatedAccept(prepareVoteFilter(cand), prepareAcceptFilter(cand), d.ballots)
		if accepted {
			// update prepared ballot
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
				log.Infof("accept prepared ballot updated")
				d.sendBallot()
			}
			return updated
		}
	}

	return false
}

// Update internal q and p ballot states with new accepted ballot
func (d *Decree) setAcceptPrepared(b *Ballot) bool {
	updated := false
	if d.pBallot != nil { // b < p
		cmp := compareBallots(b, d.pBallot)
		if cmp < 0 {
			// check whether we can update q ballot
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

// Extract unique prepared candidate ballots from statement
func (d *Decree) preparedCandidates(stmt *Statement) []*Ballot {
	// filter duplicate ballots with the same values
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

	// process ballots in descending order
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
					// check the highest accepted prepared ballot counter
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

	// sort candidates in descending order
	sort.Sort(BallotSlice(candidates))

	return candidates
}

// Confirm new ballot statement as prepared by ratifying accepted prepared ballots
func (d *Decree) confirmPrepared(stmt *Statement) bool {
	if d.currentPhase != BallotPhasePrepare || d.pBallot == nil {
		return false
	}

	candidates := d.preparedCandidates(stmt)

	log.Infof("get %d candidates for confirming prepared", len(candidates))

	// candidate for higher ballot
	var hCandidate *ultpb.Ballot

	i := 0
	for ; i < len(candidates); i++ {
		cand := candidates[i]
		// skip the ballot if it is impossible to find
		// a higher confirmed prepared ballot
		if d.hBallot != nil && compareBallots(cand, d.hBallot) <= 0 {
			break
		}

		if d.federatedRatify(prepareAcceptFilter(cand), d.ballots) {
			hCandidate = cand
			break
		}
	}

	if hCandidate != nil {
		// candidate for commit ballot
		var cCandidate *Ballot
		curb := d.currentBallot
		if curb == nil {
			curb = &Ballot{}
		}
		// find a new commit ballot
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
				// TODO(bobonovski) break or continue searching?
			}
		}
		updated := d.setConfirmPrepared(cCandidate, hCandidate)
		return updated
	}

	return false
}

// Update internal c and h ballots with new confirmed prepared ballots
func (d *Decree) setConfirmPrepared(cb *Ballot, hb *Ballot) bool {
	// save the next ballot value to use
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

// Accept commit statement for the ballot
func (d *Decree) acceptCommit(stmt *Statement) bool {
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	// create commit ballot from statement
	ballot := &Ballot{}
	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		if prepare.LC == 0 { // no commit ballot yet
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

	// make sure the new ballot is compatible with local higher ballot
	if d.currentPhase == BallotPhaseConfirm && !compatibleBallots(ballot, d.hBallot) {
		return false
	}

	// collect all the candidate counters
	counters := d.getCommitCounters(ballot)

	filter := func(l, r uint32) bool {
		return d.federatedAccept(commitVoteFilter(ballot, l, r), commitAcceptFilter(ballot, l, r), d.ballots)
	}

	lb, rb := d.findCommitInterval(counters, filter)

	log.Infof("get commit interval %d,%d for accepting commit", lb, rb)

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

// Update internal c and h ballots with new accepted commit ballots
func (d *Decree) setAcceptCommit(cb *Ballot, hb *Ballot) bool {
	// save the next ballot value to use
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

// Find lower and higher counter for commit ballot
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

// Extract commit lower and higher counters
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

// Confirm the commit ballot using federated voting
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

	log.Infof("get commit interval %d,%d for confirming commit", lb, rb)

	if lb != 0 {
		cBallot := &Ballot{Value: ballot.Value, Counter: lb}
		hBallot := &Ballot{Value: ballot.Value, Counter: rb}
		updated := d.setConfirmCommit(cBallot, hBallot)
		return updated
	}

	return false
}

// Update internal c and h ballots with confirmed commit ballot
// and trigger externalization of the commit value
func (d *Decree) setConfirmCommit(cb *Ballot, hb *Ballot) bool {
	// update commit and higher ballot
	d.cBallot = cb
	d.hBallot = hb

	d.updateBallotIfNeeded(d.hBallot)

	// change phase to externalize
	d.currentPhase = BallotPhaseExternalize

	d.sendBallot()

	// stop nomination protocol
	d.nominationStart = false

	// trigger externalization
	d.externalizeChan <- &ExternalizeValue{
		Index: d.index,
		Value: d.cBallot.Value,
	}

	return true
}

// Update the current ballot if needed
func (d *Decree) updateBallotIfNeeded(b *Ballot) bool {
	updated := false
	if d.currentBallot == nil || compareBallots(d.currentBallot, b) < 0 {
		d.updateBallot(b)
		updated = true
	}
	return updated
}

// Update the current ballot phase
func (d *Decree) updateBallotPhase(val string, force bool) bool {
	if !force && d.currentBallot != nil {
		return false
	}

	counter := uint32(1)
	if d.currentBallot != nil {
		counter = d.currentBallot.Counter + 1
	}

	log.Infow("try to update ballot phase", "counter", counter, "val", val)

	updated := d.updateBallotPhaseWithCounter(val, counter)

	return updated
}

// Update the current ballot phase with specified counter
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
		log.Infof("current ballot updated")
		d.sendBallot()
	}

	return updated
}

// Update the current ballot value
func (d *Decree) updateBallotValue(b *Ballot) bool {
	if d.currentPhase != BallotPhasePrepare && d.currentPhase != BallotPhaseConfirm {
		return false
	}

	updated := false

	if d.currentBallot == nil {
		d.updateBallot(b)
		updated = true
	} else {
		if compareBallots(d.currentBallot, b) <= 0 {
			log.Fatal("cannot update current ballot with smaller one")
		}

		if d.cBallot != nil && strings.Compare(d.cBallot.Value, b.Value) != 0 {
			return false
		}

		if compareBallots(d.currentBallot, b) <= 0 {
			d.updateBallot(b)
			updated = true
			log.Infof("current ballot updated")
		}
	}

	d.checkBallotInvariants()

	return updated
}

// Update the current ballot
func (d *Decree) updateBallot(b *Ballot) {
	if d.currentPhase == BallotPhaseExternalize {
		log.Fatal("should not update ballot in externalize phase")
	}

	if d.currentBallot != nil && compareBallots(d.currentBallot, b) > 0 {
		log.Fatal("cannot update current ballot with smaller one")
	}

	// TODO(bobonovski) copy pointer or create new one?
	d.currentBallot = b

	if d.hBallot != nil && !compatibleBallots(d.currentBallot, d.hBallot) {
		d.hBallot.Reset()
	}
}

// Check invariants of ballot states
func (d *Decree) checkBallotInvariants() {
	if d.currentBallot != nil && d.currentBallot.Counter == 0 {
		log.Fatal("current ballot is not nil but counter is zero")
	}

	if d.pBallot != nil && d.qBallot != nil {
		cond := compareBallots(d.qBallot, d.pBallot) <= 0 && !compatibleBallots(d.qBallot, d.pBallot)
		if !cond {
			log.Fatal("q ballot and p ballot invariant not satisfied")
		}
	}

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
// 1. ballot counters are in expected states
// 2. ballot values are normal and satisfy consensus constraints
func (d *Decree) validateBallot(stmt *Statement) error {
	if stmt == nil {
		return fmt.Errorf("ballot statement is nil")
	}

	fromSelf := d.nodeID == stmt.NodeID

	// value set for checking validity of ballot value
	values := mapset.NewSet()

	// validate quorum
	quorum := d.getStatementQuorum(stmt)
	if err := d.validateQuorum(quorum, 0); err != nil {
		return fmt.Errorf("validate quorum failed: %v", err)
	}

	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		// checking counter sanity
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
		// add value to set
		if prepare.B.Counter > 0 {
			values.Add(prepare.B.Value)
		}
		if prepare.P != nil {
			values.Add(prepare.P.Value)
		}
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		// check counter sanity
		if confirm.B.Counter == 0 {
			return fmt.Errorf("confirm current ballot counter should not be zero")
		}
		cond := confirm.LC <= confirm.HC && confirm.HC <= confirm.B.Counter
		if !cond {
			return fmt.Errorf("confirm ballot counters are not in expected states")
		}
		// add value to set
		values.Add(confirm.B.Value)
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		// check counter sanity
		cond := ext.B.Counter > 0 && ext.B.Counter <= ext.HC
		if !cond {
			return fmt.Errorf("externalize ballot counters are not in expected states")
		}
		// add value to set
		values.Add(ext.B.Value)
	default:
		log.Fatal(ErrUnknownStmtType)
	}

	// check values against consensus constraints
	var valueErr error
	for v := range values.Iter() {
		if err := d.validateConsensusValue(v.(string)); err != nil {
			valueErr = err
		}
	}

	return valueErr
}

// Validate consensus value
func (d *Decree) validateConsensusValue(val string) error {
	vb, err := b58.Decode(val)
	if err != nil {
		return fmt.Errorf("decode hex string failed: %v", err)
	}

	_, err = ultpb.DecodeConsensusValue(vb)
	if err != nil {
		return fmt.Errorf("decode consensus value failed: %v", err)
	}

	// TODO(bobonovski) define maybe validate state

	return nil
}

// Validate statement quorum
func (d *Decree) validateQuorum(quorum *Quorum, depth int) error {
	if depth > 2 {
		return errors.New("quorum nesting too deep")
	}

	if quorum.Threshold <= 0.0 && quorum.Threshold > 1.0 {
		return fmt.Errorf("quorum threshold out of range in depth %d", depth)
	}

	// qsize := float64(len(quorum.Validators) + len(quorum.NestQuorums))
	// threshold := int(math.Ceil(qsize * (1.0 - quorum.Threshold)))

	// check whether there exists duplicate validator in quorum
	vset := mapset.NewSet()
	for _, v := range quorum.Validators {
		if vset.Contains(v) {
			return fmt.Errorf("duplicate quorum validator %s exists", v)
		}
		vset.Add(v)
	}

	// check nested quorum
	for _, nq := range quorum.NestQuorums {
		if err := d.validateQuorum(nq, depth+1); err != nil {
			return fmt.Errorf("validate nest quorum in depth %d failed: %v", depth+1, err)
		}
	}

	return nil
}
