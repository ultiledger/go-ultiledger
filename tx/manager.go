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
	"github.com/ultiledger/go-ultiledger/exchange"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/tx/op"
	"github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/util"
)

var (
	ErrAccountNotExist    = errors.New("account not exist")
	ErrInsufficientForFee = errors.New("insufficient balance for fee")
	ErrInsufficientForTx  = errors.New("insufficient balance for tx")
	ErrInvalidSeqNum      = errors.New("invalid sequence number")
	ErrInvalidOpType      = errors.New("invalid op type")
)

var (
	// Maximum number of operations in a transactions.
	MaxTxOpList = int(100)
	// Maximum length of note in a transaction.
	MaxTxNote = int(128)
)

// ManagerContext represents contextual information TxManager needs.
type ManagerContext struct {
	NetworkID   string
	Database    db.Database
	AM          *account.Manager
	PM          *peer.Manager
	EM          *exchange.Manager
	BaseReserve int64
	Seed        string
}

func ValidateManagerContext(mc *ManagerContext) error {
	if mc == nil {
		return errors.New("tx context is nil")
	}
	if mc.NetworkID == "" {
		return errors.New("network id is empty")
	}
	if mc.Database == nil {
		return errors.New("database instance is nil")
	}
	if mc.AM == nil {
		return errors.New("account manager is nil")
	}
	if mc.PM == nil {
		return errors.New("peer manager is nil")
	}
	if mc.Seed == "" {
		return errors.New("seed is empty")
	}
	return nil
}

// Manager manages incoming tx and coordinates with ledger manager
// and consensus engine.
type Manager struct {
	networkID string

	database db.Database
	bucket   string
	txBucket string

	seed string

	baseReserve int64

	am *account.Manager
	pm *peer.Manager
	em *exchange.Manager

	// LRU cache of the tx status.
	txStatus *lru.Cache

	// Set for filtering duplicated tx.
	txSet mapset.Set

	// accountID to tx history map
	rwm      sync.RWMutex
	accTxMap map[string]*TxHistory

	// Channel for broadcasting tx.
	txChan chan *ultpb.Tx

	stopChan chan struct{}
}

// NewManager creates an instance of Manager with TxManagerContext.
func NewManager(ctx *ManagerContext) *Manager {
	if err := ValidateManagerContext(ctx); err != nil {
		log.Fatalf("tx manager context is invalid: %v", err)
	}
	tm := &Manager{
		networkID:   ctx.NetworkID,
		database:    ctx.Database,
		bucket:      "TXSTATUS",
		txBucket:    "TX",
		seed:        ctx.Seed,
		baseReserve: ctx.BaseReserve,
		am:          ctx.AM,
		pm:          ctx.PM,
		em:          ctx.EM,
		accTxMap:    make(map[string]*TxHistory),
		txChan:      make(chan *ultpb.Tx),
		stopChan:    make(chan struct{}),
	}
	err := tm.database.NewBucket(tm.bucket)
	if err != nil {
		log.Fatalf("create tx status bucket failed: %v", err)
	}
	err = tm.database.NewBucket(tm.txBucket)
	if err != nil {
		log.Fatalf("create tx bucket failed: %v", err)
	}
	cache, err := lru.New(100)
	if err != nil {
		log.Fatalf("create tx status LRU cache failed: %v", err)
	}
	tm.txStatus = cache
	return tm
}

// Start the internal event loop for tx manager.
func (tm *Manager) Start() {
	go func() {
		for {
			select {
			case tx := <-tm.txChan:
				err := tm.broadcastTx(tx)
				if err != nil {
					log.Errorf("broadcast tx failed: %v", err)
					continue
				}
			case <-tm.stopChan:
				return
			}
		}
	}()
}

// Stop tx manager by closing stopChan to notify goroutines to stop.
func (tm *Manager) Stop() {
	close(tm.stopChan)
}

// Add transaction to internal pending set.
func (tm *Manager) AddTx(txKey string, tx *ultpb.Tx) error {
	if tm.txSet.Contains(txKey) {
		return nil
	}

	acc, err := tm.am.GetAccount(tm.database, tx.AccountID)
	if err != nil {
		return fmt.Errorf("get account failed: %v", err)
	}
	if acc == nil {
		return ErrAccountNotExist
	}

	// Compute the total fees and max sequence number of the account.
	totalFees := tx.Fee
	maxSeq := tx.SeqNum
	if h, ok := tm.accTxMap[tx.AccountID]; ok {
		totalFees += h.TotalFees
		maxSeq = util.MaxUint64(maxSeq, h.MaxSeqNum)
	} else {
		tm.accTxMap[tx.AccountID] = NewTxHistory()
	}

	// Check whether tx sequence number is larger than the existing one.
	if maxSeq > tx.SeqNum {
		return errors.New("invalid sequence number")
	}

	// Check whether the account has sufficient balance.
	balance := acc.Balance - tm.baseReserve*int64(acc.EntryCount)
	if balance < totalFees {
		return errors.New("insufficient account balance")
	}

	// Check the validity of tx note.
	if len(tx.Note) > MaxTxNote {
		return errors.New("tx note is too long")
	}

	// Check the validity of op list.
	if len(tx.OpList) > MaxTxOpList {
		return errors.New("too much tx ops")
	}

	tm.rwm.Lock()
	tm.accTxMap[tx.AccountID].AddTx(txKey, tx)
	tm.rwm.Unlock()

	tm.txSet.Add(txKey)

	// Save the tx in db.
	b, err := ultpb.Encode(tx)
	if err != nil {
		return fmt.Errorf("encode tx failed: %v", err)
	}

	err = tm.database.Put(tm.txBucket, []byte(txKey), b)
	if err != nil {
		return fmt.Errorf("save tx in db failed: %v", err)
	}

	// Update tx status.
	status := &rpcpb.TxStatus{
		StatusCode: rpcpb.TxStatusCode_ACCEPTED,
	}
	err = tm.UpdateTxStatus(txKey, status)
	if err != nil {
		return fmt.Errorf("update tx status failed: %v", err)
	}

	// Broadcast the tx.
	go func() { tm.txChan <- tx }()

	return nil
}

// Apply the tx list by charging fees and applying all the ops.
func (tm *Manager) ApplyTxList(txList []*ultpb.Tx, ledgerSeqNum uint64) error {
	// Sort tx by sequence number.
	sort.Sort(TxSlice(txList))

	// Group txs by account and txs of each account are sorted
	// by sequence number in increasing order.
	accTxMap := make(map[string][]*ultpb.Tx)
	for _, tx := range txList {
		accTxMap[tx.AccountID] = append(accTxMap[tx.AccountID], tx)
	}

	// Charge tx fees.
	restTxList := make([]*ultpb.Tx, 0)
	for id, txs := range accTxMap {
		acc, err := tm.am.GetAccount(tm.database, id)
		if err != nil {
			return fmt.Errorf("get account %s failed: %v", id, err)
		}
		if acc == nil {
			// This should never happen as tx that is added to the tx manager
			// should have been checked against the existance of its corresponding
			// account. If this happens, it means there are some nodes without some
			// accounts. This situation will only happen if the consensus protocol
			// is broken.
			return ErrAccountNotExist
		}

		for i := 0; i < len(txs); i++ {
			txk, _ := ultpb.GetTxKey(txs[i])

			// Check validity of the sequence number.
			if acc.SeqNum > txs[i].SeqNum {
				status := &rpcpb.TxStatus{
					StatusCode:   rpcpb.TxStatusCode_FAILED,
					ErrorMessage: ErrInvalidSeqNum.Error(),
				}
				err = tm.UpdateTxStatus(txk, status)
				if err != nil {
					return fmt.Errorf("update tx %s status failed: %v", txk, err)
				}
				continue
			}

			// Check sufficiency of the balance.
			if acc.Balance < txs[i].Fee {
				status := &rpcpb.TxStatus{
					StatusCode:   rpcpb.TxStatusCode_FAILED,
					ErrorMessage: ErrInsufficientForFee.Error(),
				}
				err = tm.UpdateTxStatus(txk, status)
				if err != nil {
					return fmt.Errorf("update tx %s status failed: %v", txk, err)
				}
				continue
			}

			acc.Balance -= txs[i].Fee
			acc.SeqNum = txs[i].SeqNum
			restTxList = append(restTxList, txs[i])
		}

		// Update account balance.
		err = tm.am.SaveAccount(tm.database, acc)
		if err != nil {
			return fmt.Errorf("update account failed: %v", err)
		}
	}

	// TODO(bobonovski) sort the rest of the txs in more random way
	sort.Sort(TxSlice(restTxList))

	for _, tx := range restTxList {
		txk, _ := ultpb.GetTxKey(tx)

		ops, err := tm.getTxOpList(tx.OpList, tx.AccountID, ledgerSeqNum)
		if err != nil {
			status := &rpcpb.TxStatus{
				StatusCode:   rpcpb.TxStatusCode_FAILED,
				ErrorMessage: err.Error(),
			}
			err := tm.UpdateTxStatus(txk, status)
			if err != nil {
				return fmt.Errorf("update tx %s status failed: %v", txk, err)
			}
		}

		// Apply the operations in a db transaction. If any of the
		// operation failed the tx will be deemed as failed.
		dt, err := tm.database.Begin()
		if err != nil {
			return fmt.Errorf("start db transaction failed: %v", err)
		}

		var opErr error
		for _, o := range ops {
			if err := o.Apply(dt); err != nil {
				opErr = err
				dt.Rollback()
				break
			}
		}

		if opErr != nil {
			status := &rpcpb.TxStatus{
				StatusCode:   rpcpb.TxStatusCode_FAILED,
				ErrorMessage: opErr.Error(),
			}
			err := tm.UpdateTxStatus(txk, status)
			if err != nil {
				return fmt.Errorf("update tx %s status failed: %v", txk, err)
			}
		} else {
			if err := dt.Commit(); err != nil {
				return fmt.Errorf("commit changes to db failed: %v", err)
			}
		}
	}

	return nil
}

// Construct tx op list from generic op list.
func (tm *Manager) getTxOpList(opList []*ultpb.Op, accountID string, ledgerSeqNum uint64) ([]op.Op, error) {
	var ops []op.Op
	for _, o := range opList {
		switch o.OpType {
		case ultpb.OpType_CREATE_ACCOUNT:
			ca := o.GetCreateAccount()
			ops = append(ops, &op.CreateAccount{
				SrcAccountID: accountID,
				DstAccountID: ca.AccountID,
				Balance:      ca.Balance,
				BaseReserve:  tm.baseReserve,
				LedgerSeqNum: ledgerSeqNum,
			})
		case ultpb.OpType_PAYMENT:
			payment := o.GetPayment()
			ops = append(ops, &op.Payment{
				AM:           tm.am,
				EM:           tm.em,
				SrcAccountID: accountID,
				DstAccountID: payment.AccountID,
				Asset:        payment.Asset,
				Amount:       payment.Amount,
			})
		default:
			return nil, fmt.Errorf("received invalid op type: %v", o.OpType)
		}
	}
	return ops, nil
}

// Get concatenated tx list of each account.
func (tm *Manager) GetTxList() []*ultpb.Tx {
	var txList []*ultpb.Tx

	tm.rwm.RLock()
	defer tm.rwm.RUnlock()
	for _, txh := range tm.accTxMap {
		txs := txh.GetTxList()
		txList = append(txList, txs...)
	}

	return txList
}

// Delete tx list from the manager and update internal fields.
func (tm *Manager) DeleteTxList(txList []*ultpb.Tx) {
	accTxMap := make(map[string][]string)
	for _, tx := range txList {
		txKey, _ := ultpb.GetTxKey(tx)
		accTxMap[tx.AccountID] = append(accTxMap[tx.AccountID], txKey)
	}

	for acc, txList := range accTxMap {
		tm.rwm.Lock()
		th, ok := tm.accTxMap[acc]
		if !ok {
			continue
		}
		th.DeleteTxList(txList)

		// clear empty tx history
		if th.Size() == 0 {
			delete(tm.accTxMap, acc)
		}
		tm.rwm.Unlock()
	}
}

// Get the status of tx.
func (tm *Manager) GetTxStatus(txKey string) (*rpcpb.TxStatus, error) {
	if tx, ok := tm.txStatus.Get(txKey); ok {
		return tx.(*rpcpb.TxStatus), nil
	}

	status := &rpcpb.TxStatus{}
	b, err := tm.database.Get(tm.bucket, []byte(txKey))
	if err != nil {
		return nil, fmt.Errorf("get tx %s status failed: %v", txKey, err)
	}
	if b == nil {
		status.StatusCode = rpcpb.TxStatusCode_NOTEXIST
		return status, nil
	}

	err = pb.Unmarshal(b, status)
	if err != nil {
		return nil, err
	}

	return status, nil
}

// Get the tx.
func (tm *Manager) GetTx(txKey string) (*ultpb.Tx, error) {
	tx := &ultpb.Tx{}

	b, err := tm.database.Get(tm.txBucket, []byte(txKey))
	if err != nil {
		return nil, fmt.Errorf("get tx %s failed: %v", txKey, err)
	}
	if b == nil {
		return nil, nil
	}

	err = pb.Unmarshal(b, tx)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// Update the status of tx.
func (tm *Manager) UpdateTxStatus(txKey string, status *rpcpb.TxStatus) error {
	tm.txStatus.Add(txKey, status)

	b, err := ultpb.Encode(status)
	if err != nil {
		return fmt.Errorf("encode status failed: %v", err)
	}

	err = tm.database.Put(tm.bucket, []byte(txKey), b)
	if err != nil {
		return fmt.Errorf("save status in db failed: %v", err)
	}
	tm.txStatus.Add(txKey, status)

	return nil
}

// Broadcast the transaction through rpc.
func (tm *Manager) broadcastTx(tx *ultpb.Tx) error {
	clients := tm.pm.GetLiveClients()
	metadata := tm.pm.GetMetadata()

	payload, err := ultpb.Encode(tx)
	if err != nil {
		return fmt.Errorf("encode tx failed: %v", err)
	}

	sign, err := crypto.Sign(tm.seed, payload)
	if err != nil {
		return fmt.Errorf("sign tx failed: %v", err)
	}

	err = rpc.BroadcastTx(clients, metadata, payload, sign, tm.networkID)
	if err != nil {
		return fmt.Errorf("rpc broadcas failed: %v", err)
	}

	return nil
}
