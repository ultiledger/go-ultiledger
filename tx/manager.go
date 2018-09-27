package tx

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/deckarep/golang-set"
	pb "github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrInsufficientForFee = errors.New("insufficient balance for fee")
	ErrInsufficientForTx  = errors.New("insufficient balance for tx")
	ErrInvalidSeqNum      = errors.New("invalid sequence number")
)

// ManagerContext represents contextual information TxManager needs
type ManagerContext struct {
	Store       db.DB            // database instance
	AM          *account.Manager // account manager
	PM          *peer.Manager    // peer manager
	BaseReserve uint64           // global base reserve for an account
	Seed        string           // local node seed for signing message
}

func ValidateManagerContext(mc *ManagerContext) error {
	if mc == nil {
		return fmt.Errorf("tx context is nil")
	}
	if mc.Store == nil {
		return fmt.Errorf("database instance is nil")
	}
	if mc.AM == nil {
		return fmt.Errorf("account manager is nil")
	}
	if mc.PM == nil {
		return fmt.Errorf("peer manager is nil")
	}
	if mc.Seed == "" {
		return fmt.Errorf("seed is empty")
	}
	return nil
}

// Manager manages incoming tx and coordinate with ledger manager
// and consensus engine
type Manager struct {
	store  db.DB
	bucket string

	seed string

	baseReserve uint64

	am *account.Manager
	pm *peer.Manager

	// transactions status
	txStatus *lru.Cache

	// transactions waiting to be include in the ledger
	txSet mapset.Set

	// accountID to tx history map
	rwm      sync.RWMutex
	accTxMap map[string]*TxHistory

	// tx to accountID map for convenient handling
	// tx that need to be deleted
	txAccMap map[string]string

	// channel for broadcasting tx
	txChan chan *ultpb.Tx
	// channel for stopping goroutines
	stopChan chan struct{}
}

// NewManager creates an instance of Manager with TxManagerContext
func NewManager(ctx *ManagerContext) *Manager {
	if err := ValidateManagerContext(ctx); err != nil {
		log.Fatalf("tx manager context is invalid: %v", err)
	}
	m := &Manager{
		store:       ctx.Store,
		bucket:      "TX",
		seed:        ctx.Seed,
		baseReserve: ctx.BaseReserve,
		am:          ctx.AM,
		pm:          ctx.PM,
		accTxMap:    make(map[string]*TxHistory),
		txChan:      make(chan *ultpb.Tx),
		stopChan:    make(chan struct{}),
	}
	err := m.store.CreateBucket(m.bucket)
	if err != nil {
		log.Fatalf("create tx bucket failed: %v", err)
	}
	cache, err := lru.New(1000)
	if err != nil {
		log.Fatalf("create tx status LRU cache failed: %v", err)
	}
	m.txStatus = cache
	return m
}

// Start the internal event loop for tx manager
func (m *Manager) Start() {
	go func() {
		for {
			select {
			case tx := <-m.txChan:
				err := m.broadcastTx(tx)
				if err != nil {
					log.Errorf("broadcast tx failed: %v", err)
					continue
				}
			case <-m.stopChan:
				return
			}
		}
	}()
}

// Stop tx manager by closing stopChan to notify goroutines to stop
func (m *Manager) Stop() {
	close(m.stopChan)
}

// Find the max between two uint64 values
func MaxUint64(x uint64, y uint64) uint64 {
	if x >= y {
		return x
	}
	return y
}

// Add transaction to internal pending set
func (m *Manager) AddTx(txKey string, tx *ultpb.Tx) error {
	if m.txSet.Contains(txKey) {
		// directly return for duplicate tx
		return nil
	}

	// get the account information
	acc, err := m.am.GetAccount(tx.AccountID)
	if err != nil {
		return fmt.Errorf("get account %s failed: %v", tx.AccountID, err)
	}

	// compute the total fees and max sequence number
	totalFees := tx.Fee
	maxSeq := tx.SequenceNumber
	if h, ok := m.accTxMap[tx.AccountID]; ok {
		totalFees += h.TotalFees
		maxSeq = MaxUint64(maxSeq, h.MaxSeqNum)
	} else {
		m.accTxMap[tx.AccountID] = NewTxHistory()
	}

	// check whether tx sequence number is larger than existing one
	if maxSeq > tx.SequenceNumber {
		return fmt.Errorf("account %s seqnum mismatch: max %d, input %d", tx.AccountID, maxSeq, tx.SequenceNumber)
	}

	// check whether the accounts has sufficient balance
	balance := acc.Balance - m.baseReserve*uint64(acc.EntryCount)
	if balance < totalFees {
		return fmt.Errorf("account %s insufficient balance", tx.AccountID)
	}

	m.rwm.Lock()
	m.accTxMap[tx.AccountID].AddTx(txKey, tx)
	m.txAccMap[txKey] = tx.AccountID
	m.rwm.Unlock()

	m.txSet.Add(txKey)

	// change tx status
	status := &rpcpb.TxStatus{
		StatusCode: rpcpb.TxStatusCode_ACCEPTED,
	}
	err = m.UpdateTxStatus(txKey, status)
	if err != nil {
		return fmt.Errorf("update tx status failed: %v", err)
	}

	// add tx to broadcast channel
	go func() { m.txChan <- tx }()

	return nil
}

// Apply the tx list by charging fees and applying all the ops
func (m *Manager) ApplyTxList(txList []*ultpb.Tx) error {
	// sort tx by sequence number
	sort.Sort(TxSlice(txList))

	// group tx by account and txs of each account is sorted
	// by sequence number in increasing order
	accTxMap := make(map[string][]*ultpb.Tx)
	for _, tx := range txList {
		accTxMap[tx.AccountID] = append(accTxMap[tx.AccountID], tx)
	}

	// charge tx fees
	for id, txs := range accTxMap {
		acc, err := m.am.GetAccount(id)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		// cache normal tx
		txList := make([]*ultpb.Tx, 0)

		i := 0
		for ; i < len(txs); i++ {
			txk, _ := ultpb.GetTxKey(txs[i])

			// check validity of sequence number
			if acc.SequenceNumber > txs[i].SequenceNumber {
				status := &rpcpb.TxStatus{
					StatusCode:   rpcpb.TxStatusCode_FAILED,
					ErrorMessage: ErrInvalidSeqNum.Error(),
				}
				err = m.UpdateTxStatus(txk, status)
				if err != nil {
					return fmt.Errorf("update tx %s status failed: %v", txk, err)
				}
				continue
			}

			// check sufficiency of balance
			if acc.Balance < txs[i].Fee {
				status := &rpcpb.TxStatus{
					StatusCode:   rpcpb.TxStatusCode_FAILED,
					ErrorMessage: ErrInsufficientForFee.Error(),
				}
				err = m.UpdateTxStatus(txk, status)
				if err != nil {
					return fmt.Errorf("update tx %s status failed: %v", txk, err)
				}
				continue
			}

			acc.Balance -= txs[i].Fee
			acc.SequenceNumber = txs[i].SequenceNumber
			txList = append(txList, txs[i])
		}

		// update account balance to charge fees
		err = m.am.UpdateAccount(acc)
		if err != nil {
			return fmt.Errorf("update account failed: %v", err)
		}

		// shrink txs of accounts to only maintain normal tx
		accTxMap[id] = txList
	}

	/*
		// apply tx ops
		for id, txs := range accTxMap {
			acc, _ := lm.am.GetAccount(id)
			for _, tx := range txs {
			}
		}
	*/

	return nil
}

// Get concatenated tx list of each account
func (m *Manager) GetTxList() []*ultpb.Tx {
	var txList []*ultpb.Tx

	m.rwm.RLock()
	defer m.rwm.RUnlock()
	for _, txh := range m.accTxMap {
		txs := txh.GetTxList()
		txList = append(txList, txs...)
	}

	return txList
}

// Delete tx list from the manager and update internal fields
func (m *Manager) DeleteTxList(txList []*ultpb.Tx) {
	accTxMap := make(map[string][]string)
	for _, tx := range txList {
		txKey, _ := ultpb.GetTxKey(tx)
		accTxMap[tx.AccountID] = append(accTxMap[tx.AccountID], txKey)
	}

	for acc, txList := range accTxMap {
		m.rwm.Lock()
		th, ok := m.accTxMap[acc]
		if !ok {
			continue
		}
		th.DeleteTxList(txList)

		// clear empty tx history
		if th.Size() == 0 {
			delete(m.accTxMap, acc)
		}
		m.rwm.Unlock()
	}
}

// Get the status of tx
func (m *Manager) GetTxStatus(txKey string) (*rpcpb.TxStatus, error) {
	if tx, ok := m.txStatus.Get(txKey); ok {
		return tx.(*rpcpb.TxStatus), nil
	}

	status := &rpcpb.TxStatus{}
	b, ok := m.store.Get(m.bucket, []byte(txKey))
	if !ok {
		status.StatusCode = rpcpb.TxStatusCode_NOTEXIST
		return status, nil
	}

	err := pb.Unmarshal(b, status)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// Update the status of tx
func (m *Manager) UpdateTxStatus(txKey string, status *rpcpb.TxStatus) error {
	m.txStatus.Add(txKey, status)

	b, err := ultpb.Encode(status)
	if err != nil {
		return fmt.Errorf("encode status failed: %v", err)
	}

	err = m.store.Set(m.bucket, []byte(txKey), b)
	if err != nil {
		return fmt.Errorf("save status in db failed: %v", err)
	}

	return nil
}

// Broadcast transaction through rpc broadcast
func (m *Manager) broadcastTx(tx *ultpb.Tx) error {
	clients := m.pm.GetLiveClients()
	metadata := m.pm.GetMetadata()

	payload, err := ultpb.Encode(tx)
	if err != nil {
		return fmt.Errorf("encode tx failed: %v", err)
	}

	sign, err := crypto.Sign(m.seed, payload)
	if err != nil {
		return fmt.Errorf("sign tx failed: %v", err)
	}

	err = rpc.BroadcastTx(clients, metadata, payload, sign)
	if err != nil {
		return fmt.Errorf("rpc broadcas failed: %v", err)
	}

	return nil
}
