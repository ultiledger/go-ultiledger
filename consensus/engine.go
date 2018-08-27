package consensus

import (
	"encoding/hex"
	"errors"
	"time"

	"github.com/deckarep/golang-set"
	"go.uber.org/zap"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/ledger"
	"github.com/ultiledger/go-ultiledger/peer"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
)

var (
	ErrInvalidTx           = errors.New("invalid transaction")
	ErrAccountNotFound     = errors.New("account not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidSeqNum       = errors.New("invalid sequence number")
)

// Engine is responsible for coordinating between upstream
// events and underlying consensus protocol
type Engine struct {
	store  db.DB
	bucket string

	logger *zap.SugaredLogger

	pm *peer.Manager
	am *account.Manager
	lm *ledger.Manager

	// consensus quorum
	// each time quorum is updated, its quorum hash should be recomputed accordingly
	quorum     *pb.Quorum
	quorumHash string

	// consensus protocol
	cp *ucp

	// consensus slots
	slots map[uint64]*Slot

	// transactions waiting to be include in the ledger
	txSet mapset.Set

	// accountID to txList map
	txMap map[string]*TxHistory
}

func NewEngine(d db.DB, l *zap.SugaredLogger, pm *peer.Manager, am *account.Manager, lm *ledger.Manager) *Engine {
	e := &Engine{
		store:  d,
		bucket: "ENGINE",
		logger: l,
		pm:     pm,
		am:     am,
		lm:     lm,
		cp:     newUCP(d, l),
		txSet:  mapset.NewSet(),
		txMap:  make(map[string]*TxHistory),
	}
	err := e.store.CreateBucket(e.bucket)
	if err != nil {
		e.logger.Fatal(err)
	}
	return e
}

func (e *Engine) Start(stopChan chan struct{}) {
	go func() {
		for {
			select {
			case <-stopChan:
				return
			}
		}
	}()
}

// Add transaction to internal pending set
func (e *Engine) AddTx(tx *pb.Tx) error {
	h, err := pb.SHA256Hash(tx)
	if err != nil {
		return err
	}
	if e.txSet.Contains(h) {
		return errors.New("duplicate transaction")
	}
	// get the account information
	acc, err := e.am.GetAccount(tx.AccountID)
	if err != nil {
		e.logger.Warnw("cannot find the account", "AccountID", tx.AccountID)
		return err
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
		return ErrInvalidSeqNum
	}
	// check whether the accounts has sufficient balance
	if acc.Balance-ledger.GenesisBaseReserve*uint64(acc.ItemCount) < totalFees {
		return ErrInsufficientBalance
	}
	e.txMap[tx.AccountID].AddTx(tx, h)
	e.txSet.Add(h)
	return nil
}

// Try to propose current transaction set for consensus
func (e *Engine) propose() error {
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
		return err
	}
	// construct new consensus value
	cv := &pb.ConsensusValue{
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
func (e *Engine) nominate(slotIdx uint64, prevValue *pb.ConsensusValue, currValue *pb.ConsensusValue) error {
	// get new slot
	if _, ok := e.slots[slotIdx]; !ok {
		e.slots[slotIdx] = newSlot(slotIdx, "", e.logger, e.pm)
	}
	prevEnc, err := pb.Encode(prevValue)
	if err != nil {
		return err
	}
	currEnc, err := pb.Encode(currValue)
	if err != nil {
		return err
	}
	prevEncStr := hex.EncodeToString(prevEnc)
	currEncStr := hex.EncodeToString(currEnc)
	// nominate new value for the slot
	e.slots[slotIdx].Nominate(e.quorum, e.quorumHash, prevEncStr, currEncStr)
	return nil
}
