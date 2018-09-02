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
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
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
	// for saving transaction status
	statusBucket string

	seed   string
	nodeID string

	pm *peer.Manager
	am *account.Manager
	lm *ledger.Manager

	// statement validator
	sv *Validator

	// consensus quorum
	// each time quorum is updated, its quorum hash should be recomputed accordingly
	quorum     *ultpb.Quorum
	quorumHash string

	// decrees for each round
	decrees map[uint64]*Decree

	// transactions status
	txStatus *lru.Cache

	// transactions waiting to be include in the ledger
	txSet mapset.Set

	// accountID to txList map
	txMap map[string]*TxHistory

	// channel for broadcasting statement
	statementChan chan *ultpb.Statement
	// channel for broadcasting tx
	txChan chan *ultpb.Tx
}

// NewEngine creates an instance of Engine with EngineContext
func NewEngine(ctx *EngineContext) *Engine {
	if err := ValidateEngineContext(ctx); err != nil {
		log.Fatalf("engine context is invalid: %v", err)
	}
	e := &Engine{
		store:         ctx.Store,
		bucket:        "ENGINE",
		statusBucket:  "TXSTATUS",
		seed:          ctx.Seed,
		pm:            ctx.PM,
		am:            ctx.AM,
		lm:            ctx.LM,
		sv:            NewValidator(),
		decrees:       make(map[uint64]*Decree),
		txSet:         mapset.NewSet(),
		txMap:         make(map[string]*TxHistory),
		statementChan: make(chan *ultpb.Statement),
		txChan:        make(chan *ultpb.Tx),
	}
	err := e.store.CreateBucket(e.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", e.bucket, err)
	}
	err = e.store.CreateBucket(e.statusBucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", e.statusBucket, err)
	}
	cache, err := lru.New(10000)
	if err != nil {
		log.Fatalf("create consensus engine LRU cache failed: %v", err)
	}
	e.txStatus = cache
	return e
}

func (e *Engine) Start(stopChan chan struct{}) {
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
			case <-stopChan:
				return
			}
		}
	}()
	// create seperate goroutine for listening ready statements
	go func() {
		for {
			select {
			case <-e.sv.Watch():
				seq := e.lm.NextLedgerHeaderSeq()
				for s := range e.sv.Ready(seq) {
					if _, ok := e.decrees[s.Index]; !ok {
						continue
					}
					e.decrees[s.Index].Recv(s)
				}
			case <-stopChan:
				return
			}
		}
	}()
}

// check the status of the transaction
func (e *Engine) GetTxStatus(txHash string) rpcpb.TxStatusEnum {
	if tx, ok := e.txStatus.Get(txHash); ok {
		return tx.(rpcpb.TxStatusEnum)
	}

	b, ok := e.store.Get(e.statusBucket, []byte(txHash))
	if !ok {
		return rpcpb.TxStatusEnum_NOTEXIST
	}

	sname := string(b)

	return rpcpb.TxStatusEnum(rpcpb.TxStatusEnum_value[sname])
}

// update the status of the transaction
func (e *Engine) UpdateTxStatus(txHash string, status rpcpb.TxStatusEnum) error {
	sname := rpcpb.TxStatusEnum_name[int32(status)]

	e.txStatus.Add(txHash, status)

	err := e.store.Set(e.statusBucket, []byte(txHash), []byte(sname))
	if err != nil {
		return err
	}

	return nil
}

// add transaction to internal pending set
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
	balance := acc.Balance - ledger.GenesisBaseReserve*uint64(acc.ItemCount)
	if balance < totalFees {
		return fmt.Errorf("account %s insufficient balance", tx.AccountID)
	}
	e.txMap[tx.AccountID].AddTx(tx, h)
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
	err := e.sv.Recv(stmt)
	if err != nil {
		return fmt.Errorf("send statement to validator failed: %v", err)
	}
	return nil
}

// broadcast transaction through rpc broadcast
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
		return fmt.Errorf("broadcast tx failed: %v", err)
	}

	return nil
}

// broadcast consensus message through rpc broadcast
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
		return fmt.Errorf("broadcast statement failed: %v", err)
	}

	return nil
}

// Try to propose current transaction set for consensus
func (e *Engine) Propose() error {
	// TODO(bobonovski) use sync.Pool
	txSet := TxSet{
		PrevLedgerHash: e.lm.CurrLedgerHeaderHash(),
		TxHashList:     make([]string, 0),
	}

	// append pending transactions to propose list
	for _, th := range e.txMap {
		for i, _ := range th.TxList {
			txSet.TxHashList = append(txSet.TxHashList, th.TxHashList[i])
		}
	}

	// compute hash
	hash, err := txSet.GetHash()
	if err != nil {
		return fmt.Errorf("get tx set hash failed: %v", err)
	}

	// construct new consensus value
	cv := &ultpb.ConsensusValue{
		TxListHash:  hash,
		ProposeTime: time.Now().Unix(),
	}

	// nominate new consensus value
	slotIdx := e.lm.NextLedgerHeaderSeq()
	currHeader := e.lm.CurrLedgerHeader()
	e.nominate(slotIdx, currHeader.ConsensusValue, cv)

	return nil
}

// Try to nominate the new consensus value
func (e *Engine) nominate(idx uint64, prevValue *ultpb.ConsensusValue, currValue *ultpb.ConsensusValue) error {
	// get new slot
	if _, ok := e.decrees[idx]; !ok {
		e.decrees[idx] = NewDecree(idx, "", e.quorum, e.quorumHash, e.statementChan)
	}

	prevEnc, err := ultpb.Encode(prevValue)
	if err != nil {
		return fmt.Errorf("encode prev consensus value failed: %v", err)
	}

	currEnc, err := ultpb.Encode(currValue)
	if err != nil {
		return fmt.Errorf("encode curr consensus value failed: %v", err)
	}

	prevEncStr := hex.EncodeToString(prevEnc)
	currEncStr := hex.EncodeToString(currEnc)

	// nominate new value for the slot
	e.decrees[idx].Nominate(prevEncStr, currEncStr)

	return nil
}
