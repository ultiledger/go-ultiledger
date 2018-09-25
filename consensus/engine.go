package consensus

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/deckarep/golang-set"
	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrInvalidTx           = errors.New("invalid transaction")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidSeqNum       = errors.New("invalid sequence number")
)

// EngineContext represents contextual information Engine needs
type EngineContext struct {
	Store  db.DB            // database instance
	Seed   string           // node seed
	NodeID string           // node ID
	PM     *peer.Manager    // peer manager
	AM     *account.Manager // account manager
	LM     *ledger.Manager  // ledger manager
	Quorum *ultpb.Quorum    // initial quorum parsed from config
}

func ValidateEngineContext(ec *EngineContext) error {
	if ec == nil {
		return fmt.Errorf("engine context is nil")
	}
	if ec.Seed == "" {
		return fmt.Errorf("empty node seed")
	}
	if ec.NodeID == "" {
		return fmt.Errorf("empty node ID")
	}
	if ec.PM == nil {
		return fmt.Errorf("peer manager is nil")
	}
	if ec.AM == nil {
		return fmt.Errorf("account manager is nil")
	}
	if ec.LM == nil {
		return fmt.Errorf("ledger manager is nil")
	}
	if ec.Quorum == nil {
		return fmt.Errorf("initial quorum is nil")
	}
	return nil
}

// Engine is the driver of underlying consensus protocol
type Engine struct {
	store  db.DB
	bucket string

	seed   string
	nodeID string

	pm *peer.Manager
	am *account.Manager
	lm *ledger.Manager

	// statement validator
	validator *Validator

	// consensus quorum
	// each time quorum is updated, its quorum hash should be recomputed accordingly
	quorum     *ultpb.Quorum
	quorumHash string

	// decrees for each round
	decrees map[uint64]*Decree
	// max number of decrees to remember
	maxDecrees uint64

	// transactions status
	txStatus *lru.Cache

	// transactions waiting to be include in the ledger
	txSet mapset.Set

	// accountID to txlist history map
	txMap map[string]*TxHistory

	// channel for broadcasting statement
	statementChan chan *ultpb.Statement
	// channel for broadcasting tx
	txChan chan *ultpb.Tx
	// channel for downloading txset
	txsetDownloadChan chan string
	// channel for downloading quorum
	quorumDownloadChan chan string

	// channel for listening externalized value
	externalizeChan chan *ExternalizeValue

	// channel for stopping goroutines
	stopChan chan struct{}
}

// NewEngine creates an instance of Engine with EngineContext
func NewEngine(ctx *EngineContext) *Engine {
	if err := ValidateEngineContext(ctx); err != nil {
		log.Fatalf("engine context is invalid: %v", err)
	}
	e := &Engine{
		store:              ctx.Store,
		bucket:             "ENGINE",
		seed:               ctx.Seed,
		pm:                 ctx.PM,
		am:                 ctx.AM,
		lm:                 ctx.LM,
		decrees:            make(map[uint64]*Decree),
		txSet:              mapset.NewSet(),
		txMap:              make(map[string]*TxHistory),
		statementChan:      make(chan *ultpb.Statement),
		txChan:             make(chan *ultpb.Tx),
		txsetDownloadChan:  make(chan string),
		quorumDownloadChan: make(chan string),
		stopChan:           make(chan struct{}),
	}
	// create validator
	vctx := &ValidatorContext{
		Store:              e.store,
		TxSetDownloadChan:  e.txsetDownloadChan,
		QuorumDownloadChan: e.quorumDownloadChan,
	}
	e.validator = NewValidator(vctx)
	err := e.store.CreateBucket(e.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", e.bucket, err)
	}
	return e
}

func (e *Engine) Start() {
	// goroutine for listening broadcast tasks
	go func() {
		for {
			select {
			case stmt := <-e.statementChan:
				err := e.broadcastStatement(stmt)
				if err != nil {
					log.Errorf("broadcast statement failed: %v", err)
					continue
				}
			case tx := <-e.txChan:
				err := e.broadcastTx(tx)
				if err != nil {
					log.Errorf("broadcast tx failed: %v", err)
					continue
				}
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
	// goroutine for dealing with internal events
	go func() {
		for {
			select {
			case stmt := <-e.validator.Ready():
				seq := e.lm.NextLedgerHeaderSeq()
				if stmt.Index < seq-e.maxDecrees {
					// skip old statement
					continue
				}
				if _, ok := e.decrees[stmt.Index]; !ok {
					continue
				}
				e.decrees[stmt.Index].Recv(stmt)
			case ext := <-e.externalizeChan:
				err := e.Externalize(ext.Index, ext.Value)
				if err != nil {
					log.Errorf("externalize value failed: %v", err, "index", ext.Index, "value", ext.Value)
					continue
				}
			case <-e.stopChan:
				return
			}
		}
	}()
}

// Stop the consensus engine
func (e *Engine) Stop() {
	close(e.stopChan)
	e.validator.Stop()
}

// Get the quorum of the quorum hash
func (e *Engine) GetQuorum(quorumHash string) (*Quorum, error) {
	q, ok := e.validator.GetQuorum(quorumHash)
	if !ok {
		return nil, fmt.Errorf("quorum of quorum hash %s not exist", quorumHash)
	}
	return q, nil
}

// Get the txset of the txset hash
func (e *Engine) GetTxSet(txsetHash string) (*TxSet, error) {
	txs, ok := e.validator.GetTxSet(txsetHash)
	if !ok {
		return nil, fmt.Errorf("txset of txset hash %s not exist", txsetHash)
	}
	return txs, nil
}

// Find the max between two uint64 values
func MaxUint64(x uint64, y uint64) uint64 {
	if x >= y {
		return x
	}
	return y
}

// Add transaction to internal pending set
func (e *Engine) RecvTx(tx *ultpb.Tx) error {
	h, err := ultpb.SHA256Hash(tx)
	if err != nil {
		return fmt.Errorf("compute tx hash failed: %v", err)
	}

	if e.txSet.Contains(h) {
		// directly return for duplicate tx
		return nil
	}

	// get the account information
	acc, err := e.am.GetAccount(tx.AccountID)
	if err != nil {
		return fmt.Errorf("get account %s failed: %v", tx.AccountID, err)
	}

	// compute the total fees and max sequence number
	totalFees := tx.Fee
	maxSeq := tx.SequenceNumber
	if history, ok := e.txMap[tx.AccountID]; ok {
		totalFees += history.TotalFees
		maxSeq = MaxUint64(maxSeq, history.MaxSeqNum)
	} else {
		e.txMap[tx.AccountID] = NewTxHistory()
	}

	// check whether tx sequence number is larger than existing one
	if maxSeq > tx.SequenceNumber {
		return fmt.Errorf("account %s seqnum mismatch: max %d, input %d", tx.AccountID, maxSeq, tx.SequenceNumber)
	}

	// check whether the accounts has sufficient balance
	balance := acc.Balance - ledger.GenesisBaseReserve*uint64(acc.EntryCount)
	if balance < totalFees {
		return fmt.Errorf("account %s insufficient balance", tx.AccountID)
	}
	e.txMap[tx.AccountID].AddTx(tx)
	e.txSet.Add(h)

	// add tx to broadcast channel
	go func() { e.txChan <- tx }()

	return nil
}

// RecvStatement deals with received broadcast statement
func (e *Engine) RecvStatement(stmt *ultpb.Statement) error {
	// ignore own message
	if stmt.NodeID == e.nodeID {
		return nil
	}

	// send statement to validator for fetching transaction set
	// and quorum of the corresponding node
	err := e.validator.Recv(stmt)
	if err != nil {
		return fmt.Errorf("send statement to validator failed: %v", err)
	}
	return nil
}

// Broadcast transaction through rpc broadcast
func (e *Engine) broadcastTx(tx *ultpb.Tx) error {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	payload, err := ultpb.Encode(tx)
	if err != nil {
		return fmt.Errorf("encode tx failed: %v", err)
	}

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return fmt.Errorf("sign tx failed: %v", err)
	}

	err = rpc.BroadcastTx(clients, metadata, payload, sign)
	if err != nil {
		return fmt.Errorf("rpc broadcas failed: %v", err)
	}

	return nil
}

// Broadcast consensus message through rpc broadcast
func (e *Engine) broadcastStatement(stmt *ultpb.Statement) error {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	payload, err := ultpb.Encode(stmt)
	if err != nil {
		return fmt.Errorf("encode statement failed: %v", err)
	}

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return fmt.Errorf("sign statement failed: %v", err)
	}

	err = rpc.BroadcastStatement(clients, metadata, payload, sign)
	if err != nil {
		return fmt.Errorf("rpc broadcast failed: %v", err)
	}

	return nil
}

// Query quorum infomation from peers
func (e *Engine) queryQuorum(quorumHash string) (*Quorum, error) {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	payload := []byte(quorumHash)

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return nil, fmt.Errorf("sign statement failed: %v", err)
	}

	quorum, err := rpc.QueryQuorum(clients, metadata, payload, sign)
	if err != nil {
		return nil, fmt.Errorf("rpc query failed: %v", err)
	}

	// check compatibility of quorum and its hash
	hash, err := ultpb.SHA256Hash(quorum)
	if err != nil {
		return nil, fmt.Errorf("compute quorum hash failed: %v", err)
	}
	if hash != quorumHash {
		return nil, fmt.Errorf("hash of quorum is incompatible to quorumhash")
	}

	return quorum, nil
}

// Query txset infomation from peers
func (e *Engine) queryTxSet(txsetHash string) (*TxSet, error) {
	clients := e.pm.GetLiveClients()
	metadata := e.pm.GetMetadata()

	payload := []byte(txsetHash)

	sign, err := crypto.Sign(e.seed, payload)
	if err != nil {
		return nil, fmt.Errorf("sign statement failed: %v", err)
	}

	txset, err := rpc.QueryTxSet(clients, metadata, payload, sign)
	if err != nil {
		return nil, fmt.Errorf("rpc query failed: %v", err)
	}

	// check compatibility of quorum and its hash
	hash, err := ultpb.SHA256Hash(txset)
	if err != nil {
		return nil, fmt.Errorf("compute txset hash failed: %v", err)
	}
	if hash != txsetHash {
		return nil, fmt.Errorf("hash of txset is incompatible to quorumhash")
	}

	return txset, nil
}

// Try to propose current transaction set for consensus
func (e *Engine) Propose() error {
	// TODO(bobonovski) use sync.Pool
	txSet := &TxSet{
		PrevLedgerHash: e.lm.CurrLedgerHeaderHash(),
	}

	// append pending transactions to propose list
	for _, th := range e.txMap {
		for _, tx := range th.TxList {
			txSet.TxList = append(txSet.TxList, tx)
		}
	}

	// compute hash
	hash, err := ultpb.GetTxSetHash(txSet)
	if err != nil {
		return fmt.Errorf("get tx set hash failed: %v", err)
	}

	// construct new consensus value
	cv := &ultpb.ConsensusValue{
		TxSetHash:   hash,
		ProposeTime: time.Now().Unix(),
	}
	cvb, err := ultpb.Encode(cv)
	if err != nil {
		return fmt.Errorf("encode consensus value failed: %v", err)
	}
	cvStr := hex.EncodeToString(cvb)

	// nominate new consensus value
	slotIdx := e.lm.NextLedgerHeaderSeq()
	currHeader := e.lm.CurrLedgerHeader()
	e.nominate(slotIdx, currHeader.ConsensusValue, cvStr)

	return nil
}

// Nominate a new consensus value for specified decree
func (e *Engine) nominate(idx uint64, prevValue string, currValue string) error {
	// get new slot
	if _, ok := e.decrees[idx]; !ok {
		decreeCtx := &DecreeContext{
			Index:           idx,
			NodeID:          e.nodeID,
			LM:              e.lm,
			Validator:       e.validator,
			StmtChan:        e.statementChan,
			ExternalizeChan: e.externalizeChan,
		}
		e.decrees[idx] = NewDecree(decreeCtx)
	}

	// nominate new value for the slot
	e.decrees[idx].Nominate(prevValue, currValue)

	return nil
}

// Externalize a consensus value with decree index
func (e *Engine) Externalize(idx uint64, value string) error {
	// skip old externalized value
	if idx < e.lm.NextLedgerHeaderSeq() {
		return nil
	}

	// decode consensus value
	b, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("hex decode consensus value failed: %v", err)
	}
	cv, err := ultpb.DecodeConsensusValue(b)
	if err != nil {
		return fmt.Errorf("decode consensus value failed: %v", err)
	}

	txset, ok := e.validator.GetTxSet(cv.TxSetHash)
	if !ok {
		return fmt.Errorf("get txset %s failed", cv.TxSetHash)
	}

	// send value to ledger manager
	err = e.lm.RecvExtVal(idx, value, txset)
	if err != nil {
		return fmt.Errorf("externalize value in ledger manager failed: %v", err)
	}

	// TODO(bobonovski) change the status of pending transactions

	return nil
}
