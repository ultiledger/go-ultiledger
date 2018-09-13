package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/deckarep/golang-set"
	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// Validator validates incoming consensus messages and
// requests missing information of transaction and quorum
// from other peers.
type Validator struct {
	store  db.DB
	bucket string

	// statements which are downloading
	rwm       sync.RWMutex
	downloads map[string]*Statement

	// set of received statements
	statements mapset.Set

	// quorum hash to quorum LRU cache
	quorumCache *lru.Cache
	// tx list hash to tx list LRU cache
	txListCache *lru.Cache

	// channel for sending quorum download task
	quorumDownloadChan chan<- string
	// channel for sending txlist download task
	txlistDownloadChan chan<- string

	// notify engine new statements are ready
	readyChan chan *Statement
	// channel for downloding missing info of statement
	downloadChan chan *Statement
	// stop channel
	stopChan chan struct{}
}

func NewValidator() *Validator {
	v := &Validator{
		downloads:    make(map[string]*Statement),
		statements:   mapset.NewSet(),
		readyChan:    make(chan *Statement, 100),
		downloadChan: make(chan *Statement, 100),
		stopChan:     make(chan struct{}),
	}
	qc, err := lru.New(1000)
	if err != nil {
		log.Fatalf("create quorum LRU cache failed: %v", err)
	}
	v.quorumCache = qc

	tc, err := lru.New(1000)
	if err != nil {
		log.Fatalf("create tx list LRU cache failed: %v", err)
	}
	v.txListCache = tc

	// listening for download task
	go v.download()
	// monitor for downloaded statements
	go v.monitor()

	return v
}

// Ready retrives ready statements with decree index less than
// and equal to input idx.
func (v *Validator) Ready() <-chan *Statement {
	stmtChan := make(chan *Statement)
	go func() {
		for stmt := range v.readyChan {
			select {
			case stmtChan <- stmt:
			case <-v.stopChan:
				return
			}
		}
	}()
	return stmtChan
}

// Receive new statement
func (v *Validator) Recv(stmt *Statement) error {
	if stmt == nil {
		return nil
	}

	// filter duplicate stmt
	hash, err := ultpb.SHA256Hash(stmt)
	if err != nil {
		return fmt.Errorf("compute statement hash failed: %v", err)
	}
	if v.statements.Contains(hash) {
		return nil
	}
	v.statements.Add(hash)

	// validate statement
	valid, err := v.validate(stmt)
	if err != nil {
		return fmt.Errorf("validate statement failed: %v", "index", stmt.Index, "nodeID", stmt.NodeID)
	}

	if valid {
		v.readyChan <- stmt
	} else {
		v.downloadChan <- stmt
		// save statement to downloads
		v.rwm.Lock()
		v.downloads[hash] = stmt
		v.rwm.Unlock()
	}

	return nil
}

// Receive downloaded quorum and save it in db and cache
func (v *Validator) RecvQuorum(quorumHash string, quorum *Quorum) error {
	// encode quorum to pb format
	qb, err := ultpb.Encode(quorum)
	if err != nil {
		return fmt.Errorf("encode quorum failed: %v", err)
	}

	// save the quorum in db first
	err = v.store.Set(v.bucket, []byte(quorumHash), qb)
	if err != nil {
		return fmt.Errorf("save quorum to db failed: %v", err)
	}

	v.quorumCache.Add(quorumHash, quorum)

	return nil
}

// Receive downloaded tx list and save it to db and cache
func (v *Validator) RecvTxList(txListHash string, txList []string) error {
	txs := strings.Join(txList, ",")

	// save the tx list in db first
	err := v.store.Set(v.bucket, []byte(txListHash), []byte(txs))
	if err != nil {
		return fmt.Errorf("save tx list to db failed: %v", err)
	}

	v.txListCache.Add(txListHash, txs)

	return nil
}

// Get the quorum of the corresponding quorum hash,
func (v *Validator) GetQuorum(quorumHash string) (*Quorum, bool) {
	if q, ok := v.quorumCache.Get(quorumHash); ok {
		return q.(*Quorum), true
	}

	qb, ok := v.store.Get(v.bucket, []byte(quorumHash))
	if !ok {
		return nil, false
	}

	quorum, err := ultpb.DecodeQuorum(qb)
	if err != nil {
		log.Errorf("decode quorum failed: %v", err, "quorumHash", quorumHash)
		return nil, false
	}

	// cache the quorum
	v.quorumCache.Add(quorumHash, quorum)

	return quorum, true
}

// Get the tx list of the corresponding tx list
func (v *Validator) GetTxList(txListHash string) ([]string, bool) {
	if q, ok := v.txListCache.Get(txListHash); ok {
		return q.([]string), true
	}

	txb, ok := v.store.Get(v.bucket, []byte(txListHash))
	if !ok {
		return nil, false
	}

	// the tx list is encoded by joining hash with comma
	txList := strings.Split(string(txb), ",")

	// cache the tx list
	v.txListCache.Add(txListHash, txList)

	return txList, true
}

// Monitor downloaded statement and dispatch to ready channel
func (v *Validator) monitor() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			v.rwm.RLock()
			for h, stmt := range v.downloads {
				// no need to check error here
				valid, _ := v.validate(stmt)
				if valid {
					log.Infof("statement %s is ready with full info", h)
					v.readyChan <- stmt
					delete(v.downloads, h)
				}
			}
			v.rwm.RUnlock()
		case <-v.stopChan:
			return
		}
	}
}

// Download missing info of the statement
func (v *Validator) download() {
	for {
		select {
		case stmt := <-v.downloadChan:
			// no need to check error since we have already passed the validate check
			quorumHash, _ := extractQuorumHash(stmt)
			if _, ok := v.GetQuorum(quorumHash); !ok {
				v.quorumDownloadChan <- quorumHash
			}

			txListHashes, _ := extractTxListHash(stmt)
			for _, txListHash := range txListHashes {
				_, ok := v.GetTxList(txListHash)
				if !ok {
					v.txlistDownloadChan <- txListHash
				}
			}
		case <-v.stopChan:
			return
		}
	}
}

// Validate statement bying check whether we have its quorum and tx list.
// If the return value is nil, it indicates the statement is ready.
func (v *Validator) validate(stmt *Statement) (bool, error) {
	quorumHash, err := extractQuorumHash(stmt)
	if err != nil {
		return false, fmt.Errorf("extract quorum hash from statement failed: %v", err)
	}

	if _, ok := v.GetQuorum(quorumHash); !ok {
		return false, nil
	}

	txListHashes, err := extractTxListHash(stmt)
	if err != nil {
		return false, fmt.Errorf("extract tx list hash from statement failed: %v", err)
	}

	for _, txListHash := range txListHashes {
		_, ok := v.GetTxList(txListHash)
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

// Extract quorum hash from statement
func extractQuorumHash(stmt *Statement) (string, error) {
	if stmt == nil {
		return "", errors.New("statement is nil")
	}
	var hash string
	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		nom := stmt.GetNominate()
		hash = nom.QuorumHash
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		hash = prepare.QuorumHash
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		hash = confirm.QuorumHash
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		hash = ext.QuorumHash
	default:
		log.Fatal(ErrUnknownStmtType)
	}
	return hash, nil
}

// Extract list of tx set hash from statement
func extractTxListHash(stmt *Statement) ([]string, error) {
	if stmt == nil {
		return nil, errors.New("statement is nil")
	}
	var hashes []string
	switch stmt.StatementType {
	case ultpb.StatementType_NOMINATE:
		nom := stmt.GetNominate()
		for _, v := range nom.VoteList {
			b, err := hex.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("decode hex string failed: %v", err)
			}
			cv, err := ultpb.DecodeConsensusValue(b)
			if err != nil {
				return nil, fmt.Errorf("decode consensus value failed: %v", err)
			}
			hashes = append(hashes, cv.TxSetHash)
		}
		for _, a := range nom.AcceptList {
			b, err := hex.DecodeString(a)
			if err != nil {
				return nil, fmt.Errorf("decode hex string failed: %v", err)
			}
			cv, err := ultpb.DecodeConsensusValue(b)
			if err != nil {
				return nil, fmt.Errorf("decode consensus value failed: %v", err)
			}
			hashes = append(hashes, cv.TxSetHash)
		}
	case ultpb.StatementType_PREPARE:
		fallthrough
	case ultpb.StatementType_CONFIRM:
		fallthrough
	case ultpb.StatementType_EXTERNALIZE:
		ballot, err := extractWorkingBallot(stmt)
		if err != nil {
			return nil, fmt.Errorf("extract working ballot failed: %v", err)
		}
		b, err := hex.DecodeString(ballot.Value)
		if err != nil {
			return nil, fmt.Errorf("decode hex string failed: %v", err)
		}
		cv, err := ultpb.DecodeConsensusValue(b)
		if err != nil {
			return nil, fmt.Errorf("decode consensus value failed: %v", err)
		}
		hashes = append(hashes, cv.TxSetHash)
	default:
		return nil, errors.New("unknown statement type")
	}
	return hashes, nil
}

// Extract working ballot according to statement type
func extractWorkingBallot(stmt *Statement) (*Ballot, error) {
	if stmt == nil {
		return nil, errors.New("statement is nil")
	}
	var ballot *Ballot
	switch stmt.StatementType {
	case ultpb.StatementType_PREPARE:
		prepare := stmt.GetPrepare()
		ballot = prepare.B
	case ultpb.StatementType_CONFIRM:
		confirm := stmt.GetConfirm()
		ballot = &Ballot{Value: confirm.B.Value, Counter: confirm.LC}
	case ultpb.StatementType_EXTERNALIZE:
		ext := stmt.GetExternalize()
		ballot = ext.B
	default:
		log.Fatal(ErrUnknownStmtType)
	}
	return ballot, nil
}
