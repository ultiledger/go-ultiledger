package consensus

import (
	"errors"
	"fmt"
	"time"

	b58 "github.com/mr-tron/base58/base58"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/tx"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrInvalidTx           = errors.New("invalid transaction")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidSeqNum       = errors.New("invalid sequence number")
)

// EngineContext represents contextual information Engine needs.
type EngineContext struct {
	NetworkID  string
	Database   db.Database
	Seed       string
	NodeID     string
	Role       string
	MaxDecrees uint64
	PM         *peer.Manager
	AM         *account.Manager
	LM         *ledger.Manager
	TM         *tx.Manager
	// Initial quorum parsed from config file.
	Quorum *ultpb.Quorum
	// Interval of consensus proposition in seconds.
	ProposeInterval int
}

func ValidateEngineContext(ec *EngineContext) error {
	if ec == nil {
		return errors.New("engine context is nil")
	}
	if ec.NetworkID == "" {
		return errors.New("network id is empty")
	}
	if ec.Seed == "" {
		return errors.New("empty node seed")
	}
	if ec.NodeID == "" {
		return errors.New("empty node ID")
	}
	if ec.Role == "" {
		return errors.New("empty node role")
	}
	if ec.MaxDecrees == 0 {
		return errors.New("max decrees is zero")
	}
	if ec.PM == nil {
		return errors.New("peer manager is nil")
	}
	if ec.AM == nil {
		return errors.New("account manager is nil")
	}
	if ec.LM == nil {
		return errors.New("ledger manager is nil")
	}
	if ec.TM == nil {
		return errors.New("tx manager is nil")
	}
	if ec.Quorum == nil {
		return errors.New("initial quorum is nil")
	}
	if ec.ProposeInterval <= 0 {
		return errors.New("propose interval is invalid")
	}
	return nil
}

// Engine is the driver of underlying federated consensus protocol.
type Engine struct {
	networkID string

	database db.Database
	bucket   string

	seed   string
	nodeID string
	role   string

	pm *peer.Manager
	am *account.Manager
	lm *ledger.Manager
	tm *tx.Manager

	// Validator of consensus statements.
	validator *Validator

	// Quorum of the local node and its hash. Note that each time quorum is updated,
	// its quorum hash should be recomputed accordingly.
	quorum     *ultpb.Quorum
	quorumHash string

	// Decrees of each round.
	decrees map[uint64]*Decree
	// Max number of historical decrees to remember. Decree which has maxDecrees of
	// difference with the current decree will be skipped for subsequent processing.
	maxDecrees uint64

	// Channel for broadcasting statements.
	statementChan chan *ultpb.Statement
	// Ticker for rebroadcasting statements,
	rebroadcastTicker *time.Ticker
	// Channel for broadcasting transactions.
	txChan chan *ultpb.Tx
	// Channel for downloading txset.
	txsetDownloadChan chan string
	// Channel for downloading quorum.
	quorumDownloadChan chan string

	// Channel for listening externalized consensus value.
	externalizeChan chan *ExternalizeValue

	// Channel for listening propose signal.
	proposeChan   chan struct{}
	proposeTicker *time.Ticker

	// Channel for stopping goroutines.
	stopChan chan struct{}
}

// NewEngine creates an instance of Engine with EngineContext.
func NewEngine(ctx *EngineContext) *Engine {
	if err := ValidateEngineContext(ctx); err != nil {
		log.Fatalf("engine context is invalid: %v", err)
	}

	quorumHash, err := ultpb.SHA256Hash(ctx.Quorum)
	if err != nil {
		log.Fatalf("compute quorum hash failed: %v", err)
	}

	e := &Engine{
		networkID:          ctx.NetworkID,
		database:           ctx.Database,
		bucket:             "ENGINE",
		nodeID:             ctx.NodeID,
		role:               ctx.Role,
		seed:               ctx.Seed,
		pm:                 ctx.PM,
		am:                 ctx.AM,
		lm:                 ctx.LM,
		tm:                 ctx.TM,
		maxDecrees:         ctx.MaxDecrees,
		quorum:             ctx.Quorum,
		quorumHash:         quorumHash,
		decrees:            make(map[uint64]*Decree),
		statementChan:      make(chan *ultpb.Statement),
		rebroadcastTicker:  time.NewTicker(time.Second),
		txChan:             make(chan *ultpb.Tx),
		txsetDownloadChan:  make(chan string),
		quorumDownloadChan: make(chan string),
		externalizeChan:    make(chan *ExternalizeValue),
		proposeChan:        make(chan struct{}),
		proposeTicker:      time.NewTicker(time.Second * time.Duration(ctx.ProposeInterval)),
		stopChan:           make(chan struct{}),
	}

	// Create a validator.
	vctx := &ValidatorContext{
		Database:           e.database,
		LM:                 e.lm,
		MaxDecrees:         e.maxDecrees,
		TxSetDownloadChan:  e.txsetDownloadChan,
		QuorumDownloadChan: e.quorumDownloadChan,
	}
	e.validator = NewValidator(vctx)

	err = e.database.NewBucket(e.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", e.bucket, err)
	}

	return e
}

func (e *Engine) Start() {
	// Goroutine for processing network messages.
	go func() {
		for {
			select {
			case stmt := <-e.statementChan:
				err := e.broadcastStatement(stmt)
				if err != nil {
					log.Errorf("broadcast statement failed: %v", err)
					continue
				}
				log.Debugw("broadcast statement succeeded", "index", stmt.Index, "type", stmt.StatementType)
			case txsetHash := <-e.txsetDownloadChan:
				txset, err := e.queryTxSet(txsetHash)
				if err != nil {
					log.Errorf("query txset failed: %v", err)
					continue
				}
				err = e.validator.RecvTxSet(txsetHash, txset)
				if err != nil {
					log.Errorf("send txset to validator failed: %v", err)
					continue
				}
			case quorumHash := <-e.quorumDownloadChan:
				quorum, err := e.queryQuorum(quorumHash)
				if err != nil {
					log.Errorf("query quorum failed: %v", err)
					continue
				}
				err = e.validator.RecvQuorum(quorumHash, quorum)
				if err != nil {
					log.Errorf("send quorum to validator failed: %v", err)
					continue
				}
			case <-e.stopChan:
				return
			}
		}
	}()
	// Goroutine for processing internal events.
	go func() {
		for {
			select {
			case stmt := <-e.validator.Ready():
				log.Debugw("recv ready statement", "nodeID", stmt.NodeID, "index", stmt.Index, "type", stmt.StatementType)
				seq := e.lm.NextLedgerHeaderSeq()
				// Skip old statement.
				if stmt.Index+e.maxDecrees < seq {
					log.Warnw("received an old statement", "index", stmt.Index, "ledgerSeq", seq)
					continue
				}
				if _, ok := e.decrees[stmt.Index]; !ok {
					continue
				}
				err := e.decrees[stmt.Index].Recv(stmt)
				if err != nil {
					log.Errorf("received statement from validator failed: %v", err)
					continue
				}
			case <-e.proposeTicker.C:
				log.Debug("start to propose new value")
				e.proposeChan <- struct{}{}
			case <-e.stopChan:
				log.Debug("stop internal events goroutine")
				return
			}
		}
	}()
	// Goroutine for proposing a new consensus value and
	// processing externalizd value.
	go func() {
		for {
			select {
			case ext := <-e.externalizeChan:
				log.Infow("recv ext value", "index", ext.Index, "value", ext.Value)
				err := e.Externalize(ext.Index, ext.Value)
				if err != nil {
					log.Errorf("externalize value failed: %v", err, "index", ext.Index, "value", ext.Value)
					continue
				}
				err = e.Propose()
				if err != nil {
					log.Errorf("propose new consensus value failed: %v", err)
					continue
				}
			case <-e.proposeChan:
				/*
					err := e.Propose()
					if err != nil {
						log.Errorf("propose new consensus value failed: %v", err)
						continue
					}
				*/
			case <-e.rebroadcastTicker.C:
				e.rebroadcast()
			case <-e.stopChan:
				return
			}
		}
	}()
}

// Stop the consensus engine.
func (e *Engine) Stop() {
	close(e.stopChan)
	e.validator.Stop()
}

// Get the quorum of the quorum hash.
func (e *Engine) GetQuorum(quorumHash string) (*Quorum, error) {
	q, err := e.validator.GetQuorum(quorumHash)
	if err != nil {
		return nil, fmt.Errorf("query quorum failed: %v", err)
	}
	return q, nil
}

// Get the txset of the txset hash.
func (e *Engine) GetTxSet(txsetHash string) (*TxSet, error) {
	txs, err := e.validator.GetTxSet(txsetHash)
	if err != nil {
		return nil, fmt.Errorf("query txset failed: %v", err)
	}
	return txs, nil
}

// Recv downloaded txset.
func (e *Engine) RecvTxSet(txsetHash string, txset *TxSet) error {
	err := e.validator.RecvTxSet(txsetHash, txset)
	if err != nil {
		return fmt.Errorf("recv txset failed: %v", err)
	}
	return nil
}

// RecvStatement deals with received broadcast statement.
func (e *Engine) RecvStatement(stmt *ultpb.Statement) error {
	// Ignore own message.
	if stmt.NodeID == e.nodeID {
		return nil
	}

	// Send statement to validator for fetching transaction set
	// and quorum of the corresponding node.
	err := e.validator.Recv(stmt)
	if err != nil {
		return fmt.Errorf("send statement to validator failed: %v", err)
	}
	return nil
}

// Broadcast the consensus statement.
func (e *Engine) broadcastStatement(stmt *ultpb.Statement) error {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	if len(clients) == 0 {
		return errors.New("there are no live clients")
	}

	log.Debugf("get %d live clients for broadcasting", len(clients))

	payload, err := ultpb.Encode(stmt)
	if err != nil {
		return fmt.Errorf("encode statement failed: %v", err)
	}

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return fmt.Errorf("sign statement failed: %v", err)
	}

	err = rpc.BroadcastStatement(clients, metadata, payload, sign, e.networkID)
	if err != nil {
		return fmt.Errorf("rpc broadcast failed: %v", err)
	}

	return nil
}

// Rebroadcast the statements of the current decree.
func (e *Engine) rebroadcast() {
	nextLedgerSeq := e.lm.NextLedgerHeaderSeq()
	decree, ok := e.decrees[nextLedgerSeq]
	if !ok {
		return
	}
	stmts := decree.GetLatestStatements()
	log.Debugf("get %d latest statements for rebroadcasting", len(stmts))

	if len(stmts) == 0 {
		return
	}
	var err error
	for _, stmt := range stmts {
		err = e.broadcastStatement(stmt)
		if err != nil {
			log.Errorw("rebroadcast statement failed", "index", stmt.Index, "type", stmt.StatementType, "err", err.Error())
		}
	}
}

// Query quorum infomation from peers.
func (e *Engine) queryQuorum(quorumHash string) (*Quorum, error) {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	payload := []byte(quorumHash)

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return nil, fmt.Errorf("sign statement failed: %v", err)
	}

	quorum, err := rpc.QueryQuorum(clients, metadata, payload, sign, e.networkID)
	if err != nil {
		return nil, fmt.Errorf("rpc query failed: %v", err)
	}

	// Check the compatibility of quorum and its hash.
	hash, err := ultpb.SHA256Hash(quorum)
	if err != nil {
		return nil, fmt.Errorf("compute quorum hash failed: %v", err)
	}
	if hash != quorumHash {
		return nil, fmt.Errorf("hash of quorum is incompatible to quorumhash")
	}

	return quorum, nil
}

// Query txset information from peers.
func (e *Engine) queryTxSet(txsetHash string) (*TxSet, error) {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	payload := []byte(txsetHash)

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return nil, fmt.Errorf("sign statement failed: %v", err)
	}

	txset, err := rpc.QueryTxSet(clients, metadata, payload, sign, e.networkID)
	if err != nil {
		return nil, fmt.Errorf("rpc query failed: %v", err)
	}

	// Check the compatibility of txset and its hash.
	hash, err := ultpb.GetTxSetKey(txset)
	if err != nil {
		return nil, fmt.Errorf("compute txset hash failed: %v", err)
	}
	if hash != txsetHash {
		return nil, fmt.Errorf("hash of txset is incompatible")
	}

	return txset, nil
}

// Try to propose current transaction set for consensus.
func (e *Engine) Propose() error {
	// Node without the role of "validator" cannot propose
	// value for consensus.
	if e.role != "validator" {
		return errors.New("the node is not a validator")
	}
	// Only node with synced ledger could propose new values.
	if !e.lm.LedgerSynced() {
		return errors.New("the ledger is not synced")
	}

	txSet := &TxSet{
		PrevLedgerHash: e.lm.CurrLedgerHeaderHash(),
		TxList:         e.tm.GetTxList(),
	}

	// Compute the hash of the txset.
	hash, err := ultpb.GetTxSetKey(txSet)
	if err != nil {
		return fmt.Errorf("get tx set hash failed: %v", err)
	}

	// Sync txset info to validator.
	err = e.validator.RecvTxSet(hash, txSet)
	if err != nil {
		return fmt.Errorf("sync txset with validator failed: %v", err)
	}

	// Sync quorum info to validator.
	err = e.validator.RecvQuorum(e.quorumHash, e.quorum)
	if err != nil {
		return fmt.Errorf("sync quorum with validator failed: %v", err)
	}

	// Construct a new consensus value.
	cv := &ultpb.ConsensusValue{
		TxSetHash:   hash,
		ProposeTime: time.Now().Unix(),
	}
	cvb, err := ultpb.Encode(cv)
	if err != nil {
		return fmt.Errorf("encode consensus value failed: %v", err)
	}
	cvStr := b58.Encode(cvb)

	// Nominate the new consensus value.
	decreeIdx := e.lm.NextLedgerHeaderSeq()
	currHeader := e.lm.CurrLedgerHeader()

	log.Infow("nominate consensus value", "decreeIdx", decreeIdx, "txsetKey", hash, "currCV", currHeader.ConsensusValue, "newCV", cvStr)

	e.nominate(decreeIdx, currHeader.ConsensusValue, cvStr)

	return nil
}

// Nominate a new consensus value for specified decree.
func (e *Engine) nominate(idx uint64, prevValue string, currValue string) error {
	// Create a new decree if there is no existing decree with the index.
	if _, ok := e.decrees[idx]; !ok {
		decreeCtx := &DecreeContext{
			Index:           idx,
			NodeID:          e.nodeID,
			Quorum:          e.quorum,
			QuorumHash:      e.quorumHash,
			LM:              e.lm,
			Validator:       e.validator,
			StmtChan:        e.statementChan,
			ExternalizeChan: e.externalizeChan,
		}
		e.decrees[idx] = NewDecree(decreeCtx)
	}

	// Nominate a new value for the decree.
	e.decrees[idx].Nominate(prevValue, currValue, false)

	return nil
}

// Externalize a consensus value with decree index.
func (e *Engine) Externalize(idx uint64, value string) error {
	// Skip old externalized value.
	if idx < e.lm.NextLedgerHeaderSeq() {
		return nil
	}

	b, err := b58.Decode(value)
	if err != nil {
		return fmt.Errorf("hex decode consensus value failed: %v", err)
	}
	cv, err := ultpb.DecodeConsensusValue(b)
	if err != nil {
		return fmt.Errorf("decode consensus value failed: %v", err)
	}

	txset, err := e.validator.GetTxSet(cv.TxSetHash)
	if err != nil {
		return fmt.Errorf("get txset failed: %v", err)
	}
	if txset == nil {
		return errors.New("txset not exist")
	}

	// Send the value to ledger manager.
	err = e.lm.RecvExtVal(idx, value, txset)
	if err != nil {
		return fmt.Errorf("externalize value in ledger manager failed: %v", err)
	}

	// Delete transactions that have been processed.
	e.tm.DeleteTxList(txset.TxList)

	// Remove decrees that are too old.
	if idx > e.maxDecrees {
		threshold := idx - e.maxDecrees
		for i, _ := range e.decrees {
			if i <= threshold {
				delete(e.decrees, i)
			}
		}
	}

	return nil
}
