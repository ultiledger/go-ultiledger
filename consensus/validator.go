package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/deckarep/golang-set"
	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// ValidatorContext contains contextual information validator needs
type ValidatorContext struct {
	Store              db.DB // database instance
	QuorumDownloadChan chan<- string
	TxSetDownloadChan  chan<- string
}

func ValidateValidatorContext(vc *ValidatorContext) error {
	if vc == nil {
		return fmt.Errorf("validator context is nil")
	}
	if vc.Store == nil {
		return fmt.Errorf("db instance is nil")
	}
	if vc.QuorumDownloadChan == nil {
		return fmt.Errorf("quorum download chan is nil")
	}
	if vc.TxSetDownloadChan == nil {
		return fmt.Errorf("txset download chan is nil")
	}
	return nil
}

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
	// txset hash to txset LRU cache
	txsetCache *lru.Cache

	// channel for sending quorum download task
	quorumDownloadChan chan<- string
	// channel for sending txset download task
	txsetDownloadChan chan<- string
	// stop channel
	stopChan chan struct{}

	// notify engine new statements are ready
	readyChan chan *Statement
	// channel for downloding missing info of statement
	downloadChan chan *Statement
}

func NewValidator(ctx *ValidatorContext) *Validator {
	if err := ValidateValidatorContext(ctx); err != nil {
		log.Fatalf("validator context is invalid: %v", err)
	}
	v := &Validator{
		store:              ctx.Store,
		bucket:             "VALIDATOR",
		downloads:          make(map[string]*Statement),
		statements:         mapset.NewSet(),
		quorumDownloadChan: ctx.QuorumDownloadChan,
		txsetDownloadChan:  ctx.TxSetDownloadChan,
		stopChan:           make(chan struct{}),
		readyChan:          make(chan *Statement, 100),
		downloadChan:       make(chan *Statement, 100),
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
	v.txsetCache = tc

	// listening for download task
	go v.download()
	// monitor for downloaded statements
	go v.monitor()

	return v
}

// Stop the validator
func (v *Validator) Stop() {
	close(v.stopChan)
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
		return fmt.Errorf("validate statement failed: %v", err)
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

// Receive downloaded txset and save it to db and cache
func (v *Validator) RecvTxSet(txsetHash string, txset *TxSet) error {
	// encode txset to pb format
	tb, err := ultpb.Encode(txset)
	if err != nil {
		return fmt.Errorf("encode txset failed: %v", err)
	}

	// save the txset in db first
	err = v.store.Set(v.bucket, []byte(txsetHash), tb)
	if err != nil {
		return fmt.Errorf("save tx list to db failed: %v", err)
	}

	v.txsetCache.Add(txsetHash, txset)

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

// Get the txset of the corresponding txset hash
func (v *Validator) GetTxSet(txsetHash string) (*TxSet, bool) {
	if txs, ok := v.txsetCache.Get(txsetHash); ok {
		return txs.(*TxSet), true
	}

	txb, ok := v.store.Get(v.bucket, []byte(txsetHash))
	if !ok {
		return nil, false
	}

	txset, err := ultpb.DecodeTxSet(txb)
	if err != nil {
		log.Errorf("decode txset failed: %v", err, "txsetHash", txsetHash)
		return nil, false
	}

	// cache the txset
	v.txsetCache.Add(txsetHash, txset)

	return txset, true
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
			// no need to check error since we have already passed the validity check
			quorumHash, _ := extractQuorumHash(stmt)
			if _, ok := v.GetQuorum(quorumHash); !ok {
				v.quorumDownloadChan <- quorumHash
			}

			txsetHashes, _ := extractTxSetHash(stmt)
			for _, txsetHash := range txsetHashes {
				_, ok := v.GetTxSet(txsetHash)
				if !ok {
					v.txsetDownloadChan <- txsetHash
				}
			}
		case <-v.stopChan:
			return
		}
	}
}

// Validate statement by checking whether we have its quorum and tx list
func (v *Validator) validate(stmt *Statement) (bool, error) {
	quorumHash, err := extractQuorumHash(stmt)
	if err != nil {
		return false, fmt.Errorf("extract quorum hash from statement failed: %v", err)
	}

	if _, ok := v.GetQuorum(quorumHash); !ok {
		return false, nil
	}

	txsetHashes, err := extractTxSetHash(stmt)
	if err != nil {
		return false, fmt.Errorf("extract tx list hash from statement failed: %v", err)
	}

	for _, txsetHash := range txsetHashes {
		_, ok := v.GetTxSet(txsetHash)
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
func extractTxSetHash(stmt *Statement) ([]string, error) {
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
		ballot := getWorkingBallot(stmt)
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
