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
	ErrInsufficientForFee = errors.New("insufficient balance for fee")
	ErrInsufficientForTx  = errors.New("insufficient balance for tx")
	ErrInvalidSeqNum      = errors.New("invalid sequence number")
	ErrInvalidOpType      = errors.New("invalid op type")
)

// ManagerContext represents contextual information TxManager needs.
type ManagerContext struct {
	Database    db.Database       // database instance
	AM          *account.Manager  // account manager
	PM          *peer.Manager     // peer manager
	EM          *exchange.Manager // exchange manager
	BaseReserve int64             // global base reserve for an account
	Seed        string            // local node seed for signing message
}

func ValidateManagerContext(mc *ManagerContext) error {
	if mc == nil {
		return fmt.Errorf("tx context is nil")
	}
	if mc.Database == nil {
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
// and consensus engine.
type Manager struct {
	database db.Database
	bucket   string

	seed string

	baseReserve int64

	am *account.Manager
	pm *peer.Manager
	em *exchange.Manager

	// transactions status
	txStatus *lru.Cache

	// transactions waiting to be include in the ledger
	txSet mapset.Set

	// accountID to tx history map
	rwm      sync.RWMutex
	accTxMap map[string]*TxHistory

	// channel for broadcasting tx
	txChan chan *ultpb.Tx
	// channel for stopping goroutines
	stopChan chan struct{}
}

// NewManager creates an instance of Manager with TxManagerContext.
func NewManager(ctx *ManagerContext) *Manager {
	if err := ValidateManagerContext(ctx); err != nil {
		log.Fatalf("tx manager context is invalid: %v", err)
	}
	tm := &Manager{
		database:    ctx.Database,
		bucket:      "TXSTATUS",
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
		log.Fatalf("create tx bucket failed: %v", err)
	}
	cache, err := lru.New(1000)
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
		// directly return for duplicate tx
		return nil
	}

	// get the account information
	acc, err := tm.am.GetAccount(tm.database, tx.AccountID)
	if err != nil {
		return fmt.Errorf("get account %s failed: %v", tx.AccountID, err)
	}

	// compute the total fees and max sequence number
	totalFees := tx.Fee
	maxSeq := tx.SeqNum
	if h, ok := tm.accTxMap[tx.AccountID]; ok {
		totalFees += h.TotalFees
		maxSeq = util.MaxUint64(maxSeq, h.MaxSeqNum)
	} else {
		tm.accTxMap[tx.AccountID] = NewTxHistory()
	}

	// check whether tx sequence number is larger than existing one
	if maxSeq > tx.SeqNum {
		return fmt.Errorf("account %s seqnum mismatch: max %d, input %d", tx.AccountID, maxSeq, tx.SeqNum)
	}

	// check whether the accounts has sufficient balance
	balance := acc.Balance - tm.baseReserve*int64(acc.EntryCount)
	if balance < totalFees {
		return fmt.Errorf("account %s insufficient balance", tx.AccountID)
	}

	tm.rwm.Lock()
	tm.accTxMap[tx.AccountID].AddTx(txKey, tx)
	tm.rwm.Unlock()

	tm.txSet.Add(txKey)

	// change tx status
	status := &rpcpb.TxStatus{
		StatusCode: rpcpb.TxStatusCode_ACCEPTED,
	}
	err = tm.UpdateTxStatus(txKey, status)
	if err != nil {
		return fmt.Errorf("update tx status failed: %v", err)
	}

	// add tx to broadcast channel
	go func() { tm.txChan <- tx }()

	return nil
}

// Apply the tx list by charging fees and applying all the ops.
func (tm *Manager) ApplyTxList(txList []*ultpb.Tx, seqNum uint64) error {
	// sort tx by sequence number
	sort.Sort(TxSlice(txList))

	// group tx by account and txs of each account is sorted
	// by sequence number in increasing order
	accTxMap := make(map[string][]*ultpb.Tx)
	for _, tx := range txList {
		accTxMap[tx.AccountID] = append(accTxMap[tx.AccountID], tx)
	}

	// charge tx fees
	restTxList := make([]*ultpb.Tx, 0)
	for id, txs := range accTxMap {
		acc, err := tm.am.GetAccount(tm.database, id)
		if err != nil {
			return fmt.Errorf("get account failed: %v", err)
		}

		for i := 0; i < len(txs); i++ {
			txk, _ := ultpb.GetTxKey(txs[i])

			// check validity of sequence number
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

			// check sufficiency of balance
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

		// update account balance to charge fees
		err = tm.am.SaveAccount(tm.database, acc)
		if err != nil {
			return fmt.Errorf("update account failed: %v", err)
		}
	}

	// TODO(bobonovski) sort the rest of the txs in more random way
	sort.Sort(TxSlice(restTxList))

	for _, tx := range restTxList {
		txk, _ := ultpb.GetTxKey(tx)

		ops, err := tm.getTxOpList(tx.OpList, tx.AccountID, seqNum)
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

		// start db transaction
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
func (tm *Manager) getTxOpList(opList []*ultpb.Op, accountID string, seqNum uint64) ([]op.Op, error) {
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
				SeqNum:       seqNum,
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

// Broadcast transaction through rpc broadcast.
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

	err = rpc.BroadcastTx(clients, metadata, payload, sign)
	if err != nil {
		return fmt.Errorf("rpc broadcas failed: %v", err)
	}

	return nil
}
