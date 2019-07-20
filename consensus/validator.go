package consensus

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/deckarep/golang-set"
	lru "github.com/hashicorp/golang-lru"
	b58 "github.com/mr-tron/base58/base58"

	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// ValidatorContext contains contextual information validator needs.
type ValidatorContext struct {
	Database           db.Database // database instance
	LM                 *ledger.Manager
	QuorumDownloadChan chan<- string
	TxSetDownloadChan  chan<- string
}

func ValidateValidatorContext(vc *ValidatorContext) error {
	if vc == nil {
		return fmt.Errorf("validator context is nil")
	}
	if vc.Database == nil {
		return fmt.Errorf("db instance is nil")
	}
	if vc.LM == nil {
		return fmt.Errorf("ledger manager is nil")
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
	database db.Database
	bucket   string

	lm *ledger.Manager

	// Statements which are downloading.
	rwm       sync.RWMutex
	downloads map[string]*Statement

	// Set of received statements.
	statements mapset.Set

	// Cache of quorum with quorum hash as the key.
	quorumCache *lru.Cache

	// Channel for sending quorum download task.
	quorumDownloadChan chan<- string
	// Channel for sending txset download task.
	txsetDownloadChan chan<- string

	stopChan chan struct{}

	// Channel for notifying consensus engine that the statement is ready.
	readyChan chan *Statement
	// Channel for downloding missing information of the statement.
	downloadChan chan *Statement
}

func NewValidator(ctx *ValidatorContext) *Validator {
	if err := ValidateValidatorContext(ctx); err != nil {
		log.Fatalf("validator context is invalid: %v", err)
	}

	v := &Validator{
		database:           ctx.Database,
		bucket:             "VALIDATOR",
		lm:                 ctx.LM,
		downloads:          make(map[string]*Statement),
		statements:         mapset.NewSet(),
		quorumDownloadChan: ctx.QuorumDownloadChan,
		txsetDownloadChan:  ctx.TxSetDownloadChan,
		stopChan:           make(chan struct{}),
		readyChan:          make(chan *Statement, 100),
		downloadChan:       make(chan *Statement, 100),
	}

	err := v.database.NewBucket(v.bucket)
	if err != nil {
		log.Fatalf("create validator bucket failed: %v", err)
	}

	qc, err := lru.New(1000)
	if err != nil {
		log.Fatalf("create quorum LRU cache failed: %v", err)
	}
	v.quorumCache = qc

	// Listening for download tasks.
	go v.download()
	// Monitor for downloaded statements.
	go v.monitor()

	return v
}

// Stop the validator
func (v *Validator) Stop() {
	close(v.stopChan)
}

// Ready returns ready statements with full information.
func (v *Validator) Ready() <-chan *Statement {
	return v.readyChan
}

// Recv checks whether the input statement has the complete
// information including quorum and txset for the consensus
// engine to process. If all the necessary information are
// present, it will directly return the statement to ready
// channel. Otherwise, it will try to download the missing
// part of information from peers.
func (v *Validator) Recv(stmt *Statement) error {
	if stmt == nil {
		return nil
	}

	// Filter the duplicate statement using the statement hash.
	hash, err := ultpb.SHA256Hash(stmt)
	if err != nil {
		return fmt.Errorf("compute statement hash failed: %v", err)
	}
	if v.statements.Contains(hash) {
		return nil
	}
	v.statements.Add(hash)

	// Check whether the statement has all the information.
	valid, err := v.validate(stmt)
	if err != nil {
		return fmt.Errorf("validate statement failed: %v", err)
	}

	if valid {
		v.readyChan <- stmt
	} else {
		v.downloadChan <- stmt
		// Save the ongoing downloading statement.
		v.rwm.Lock()
		v.downloads[hash] = stmt
		v.rwm.Unlock()
	}

	return nil
}

// RecvQuorum receives downloaded quorum and save it.
func (v *Validator) RecvQuorum(quorumHash string, quorum *Quorum) error {
	// encode quorum to pb format
	qb, err := ultpb.Encode(quorum)
	if err != nil {
		return fmt.Errorf("encode quorum failed: %v", err)
	}

	// save the quorum in db first
	err = v.database.Put(v.bucket, []byte(quorumHash), qb)
	if err != nil {
		return fmt.Errorf("save quorum to db failed: %v", err)
	}

	v.quorumCache.Add(quorumHash, quorum)

	return nil
}

// RecvTxSet receives downloaded txset and save it.
func (v *Validator) RecvTxSet(txsetHash string, txset *TxSet) error {
	err := v.lm.AddTxSet(txsetHash, txset)
	if err != nil {
		return err
	}
	return nil
}

// Get the txset from ledger manager.
func (v *Validator) GetTxSet(txsetHash string) (*TxSet, error) {
	txset, err := v.lm.GetTxSet(txsetHash)
	if err != nil {
		return nil, err
	}
	return txset, nil
}

// Get the quorum of the corresponding quorum hash,
func (v *Validator) GetQuorum(quorumHash string) (*Quorum, error) {
	if q, ok := v.quorumCache.Get(quorumHash); ok {
		return q.(*Quorum), nil
	}

	qb, err := v.database.Get(v.bucket, []byte(quorumHash))
	if err != nil {
		return nil, fmt.Errorf("get quorum from db failed: %v", err)
	}
	if qb == nil {
		return nil, nil
	}

	quorum, err := ultpb.DecodeQuorum(qb)
	if err != nil {
		return nil, fmt.Errorf("decode quorum failed: %v", err)
	}

	// Cache the quorum.
	v.quorumCache.Add(quorumHash, quorum)

	return quorum, nil
}

// Monitor downloaded statements and dispatch them to ready channel.
func (v *Validator) monitor() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			v.rwm.RLock()
			for h, stmt := range v.downloads {
				valid, _ := v.validate(stmt)
				if valid {
					log.Debugf("statement %s is ready with full info", h)
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

// Download missing info of the statement.
func (v *Validator) download() {
	for {
		select {
		case stmt := <-v.downloadChan:
			// No need to check error as we have already passed the validity checks.
			quorumHash, _ := extractQuorumHash(stmt)
			quorum, _ := v.GetQuorum(quorumHash)
			if quorum == nil {
				v.quorumDownloadChan <- quorumHash
			}

			txsetHashes, _ := extractTxSetHash(stmt)
			for _, txsetHash := range txsetHashes {
				txset, _ := v.lm.GetTxSet(txsetHash)
				if txset == nil {
					v.txsetDownloadChan <- txsetHash
				}
			}
		case <-v.stopChan:
			return
		}
	}
}

// Validate statement by checking whether we have its quorum and txset.
func (v *Validator) validate(stmt *Statement) (bool, error) {
	quorumHash, err := extractQuorumHash(stmt)
	if err != nil {
		return false, fmt.Errorf("extract quorum hash from statement failed: %v", err)
	}

	quorum, err := v.GetQuorum(quorumHash)
	if err != nil {
		return false, fmt.Errorf("query quorum failed: %v", err)
	}
	if quorum == nil {
		return false, nil
	}

	txsetHashes, err := extractTxSetHash(stmt)
	if err != nil {
		return false, fmt.Errorf("extract tx list hash from statement failed: %v", err)
	}

	for _, txsetHash := range txsetHashes {
		txset, err := v.lm.GetTxSet(txsetHash)
		if err != nil {
			return false, fmt.Errorf("query txset failed: %v", err)
		}
		if txset == nil {
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
			b, err := b58.Decode(v)
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
			b, err := b58.Decode(a)
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
		b, err := b58.Decode(ballot.Value)
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
