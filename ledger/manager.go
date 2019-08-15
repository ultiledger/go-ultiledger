package ledger

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	lru "github.com/hashicorp/golang-lru"
	b58 "github.com/mr-tron/base58/base58"

	"github.com/ultiledger/go-ultiledger/account"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/tx"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// LedgerState represents the current states of ledger,
// the ledger is synced if it complies with the states
// received from peers or we need to trigger catch up
// process to sync with the newest states.
type LedgerState uint8

const (
	LedgerStateSynced LedgerState = iota
	LedgerStateSyncing
	LedgerStateNotSynced
)

var (
	ErrUnknownLedgerState = errors.New("unknown ledger state")
	ErrInsufficientForFee = errors.New("insufficient balance for fee")
	ErrInsufficientForTx  = errors.New("insufficient balance for tx")
	ErrInvalidSeqNum      = errors.New("invalid sequence number")
	ErrLedgerNotExist     = errors.New("ledger not exist")
)

var (
	GenesisVersion = uint32(1)
	// The sequence number of the genesis ledger header.
	GenesisSeqNum = uint64(1)
	// Maximum number of transactions in a transaction set.
	GenesisMaxTxSetSize = uint32(100)
	// There are in total 4.5 billion ULT and the smallest unit is a
	// ULU which is 1/1000000000 of a ULT. The number of tokens is
	// counted in ULU.
	GenesisTotalTokens = int64(4500000000000000000)
	// The base fee for a transaction is 1000 ULU.
	GenesisBaseFee = int64(1000)
	// The base reserve for an account (1 ULT).
	GenesisBaseReserve = int64(1000000000)
)

// ManagerContext contains contextural information the ledger manager needs.
type ManagerContext struct {
	NetworkID string
	Database  db.Database
	AM        *account.Manager
	TM        *tx.Manager
	PM        *peer.Manager
}

func ValidateManagerContext(mc *ManagerContext) error {
	if mc == nil {
		return errors.New("manager context is nil")
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
	if mc.TM == nil {
		return errors.New("tx manager is nil")
	}
	if mc.PM == nil {
		return errors.New("peer manager is nil")
	}
	return nil
}

// Ledger manager is responsible for all the operations on the ledger.
type Manager struct {
	networkID string

	database    db.Database
	bucket      string
	txsetBucket string

	downloader *Downloader
	am         *account.Manager
	tm         *tx.Manager

	// LRU cache for ledger headers.
	headers *lru.Cache

	// LRU cache for txset.
	txsetCache *lru.Cache

	// Buffer for pending close info.
	buffer *CloseInfoBuffer

	// Current state of the ledger.
	ledgerState LedgerState

	// Start timestamp of the manager.
	startTime int64
	// Timestamp of last ledger closed.
	lastCloseTime int64

	// The number of ledgers processed.
	ledgerHeaderCount int64
	// The largest consensus index that the manager met.
	largestConsensusIndex uint64

	// Previous committed ledger header.
	prevLedgerHeader *ultpb.LedgerHeader
	// Hash of previous committed ledger header.
	prevLedgerHeaderHash string

	// Current latest committed ledger header.
	currLedgerHeader *ultpb.LedgerHeader
	// Hash of current latest committed ledger header.
	currLedgerHeaderHash string

	stopChan chan struct{}
}

func NewManager(ctx *ManagerContext) *Manager {
	if err := ValidateManagerContext(ctx); err != nil {
		log.Fatalf("validate manager context failed: %v", err)
	}
	lm := &Manager{
		database:    ctx.Database,
		bucket:      "LEDGER",
		txsetBucket: "TXSET",
		am:          ctx.AM,
		tm:          ctx.TM,
		buffer:      new(CloseInfoBuffer),
		downloader:  NewDownloader(ctx.NetworkID, ctx.Database, ctx.PM),
		ledgerState: LedgerStateNotSynced,
		startTime:   time.Now().Unix(),
		stopChan:    make(chan struct{}),
	}

	cache, err := lru.New(100)
	if err != nil {
		log.Fatalf("create ledger manager LRU cache failed: %v", err)
	}
	lm.headers = cache

	txscache, err := lru.New(100)
	if err != nil {
		log.Fatalf("create txset LRU cache failed: %v", err)
	}
	lm.txsetCache = txscache

	err = lm.database.NewBucket(lm.bucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", lm.bucket, err)
	}
	err = lm.database.NewBucket(lm.txsetBucket)
	if err != nil {
		log.Fatalf("create db bucket %s failed: %v", lm.txsetBucket, err)
	}
	return lm
}

// Start the ledger manager.
func (lm *Manager) Start() {
	lm.downloader.Start()

	// Deal with downloaded ledgers
	go func() {
		for {
			select {
			case info := <-lm.downloader.Ready():
				if lm.NextLedgerHeaderSeq() != info.Index {
					log.Errorw("downloaded index incompatible ledger", "expect", lm.NextLedgerHeaderSeq(), "recv", info.Index)
				}
				if lm.CurrLedgerHeaderHash() != info.TxSet.PrevLedgerHash {
					log.Errorw("downloaded hash incompatible ledger", "expect", lm.CurrLedgerHeaderHash(), "recv", info.TxSet.PrevLedgerHash)
				}
				err := lm.closeLedger(info.Index, info.Value, info.TxSet)
				if err != nil {
					log.Errorf("close ledger %d failed: %v", info.Index, err)
				}
				// Check whether we can apply buffered closeinfos.
				if lm.buffer.Size() == 0 {
					continue
				}
				head := lm.buffer.PeekHead()
				if lm.NextLedgerHeaderSeq() != head.Index {
					continue
				}
				for {
					if lm.buffer.Size() == 0 {
						break
					}
					head = lm.buffer.PopHead()
					err := lm.closeLedger(head.Index, head.Value, head.TxSet)
					if err != nil {
						log.Errorf("close ledger %d failed: %v", head.Index, err)
						break
					}
				}
				lm.buffer.Clear()
				lm.ledgerState = LedgerStateSynced
			case <-lm.stopChan:
				return
			}
		}
	}()
}

// Stop the ledger manager.
func (lm *Manager) Stop() {
	close(lm.stopChan)
	lm.downloader.Stop()
}

// Create the genesis ledger and initialize master account.
func (lm *Manager) CreateGenesisLedger() error {
	lm.ledgerState = LedgerStateSynced
	// Close genesis ledger
	err := lm.advanceLedger(GenesisSeqNum, "", "", "")
	if err != nil {
		log.Fatalf("advance ledger failed: %v", err)
	}
	return nil
}

// Get the current latest ledger header.
func (lm *Manager) CurrLedgerHeader() *ultpb.LedgerHeader {
	// Current ledger header should always exist.
	if lm.currLedgerHeader == nil {
		log.Fatal("current ledger header is nil")
	}
	return lm.currLedgerHeader
}

// Get the hash of current latest ledger header.
func (lm *Manager) CurrLedgerHeaderHash() string {
	// Current ledger header hash should always exist.
	if lm.currLedgerHeaderHash == "" {
		log.Fatal("current ledger header hash is empty")
	}
	return lm.currLedgerHeaderHash
}

// Get the sequence number of next ledger header.
func (lm *Manager) NextLedgerHeaderSeq() uint64 {
	// The current ledger header will be nil until the
	// first ledger header close.
	if lm.currLedgerHeader == nil {
		return uint64(1)
	}
	return lm.currLedgerHeader.SeqNum + 1
}

// Check whether the ledger is synced.
func (lm *Manager) LedgerSynced() bool {
	return lm.ledgerState == LedgerStateSynced
}

// Get the ledger header by ledger sequence.
func (lm *Manager) GetLedgerHeader(seq string) (*ultpb.LedgerHeader, error) {
	var header *ultpb.LedgerHeader
	h, ok := lm.headers.Get(seq)
	if ok {
		header = h.(*ultpb.LedgerHeader)
	} else {
		lb, err := lm.database.Get(lm.bucket, []byte(seq))
		if err != nil {
			return nil, fmt.Errorf("query ledger from db failed: %v", err)
		}
		if lb == nil {
			return nil, nil
		}
		header, err = ultpb.DecodeLedgerHeader(lb)
		if err != nil {
			return nil, fmt.Errorf("decode ledger failed: %v", err)
		}
		lm.headers.Add(seq, header)
	}
	return header, nil
}

// Get the ledger by ledger sequence.
func (lm *Manager) GetLedger(seq string) (*ultpb.Ledger, error) {
	header, err := lm.GetLedgerHeader(seq)
	if err != nil {
		return nil, fmt.Errorf("get ledger header failed: %v", err)
	}
	if header == nil {
		return nil, errors.New("ledger header not exist")
	}
	// Get txset of the ledger.
	txset, err := lm.GetTxSet(header.TxSetHash)
	if err != nil {
		return nil, fmt.Errorf("get txset failed: %v", err)
	}
	ledger := &ultpb.Ledger{
		LedgerHeader: header,
		TxSet:        txset,
	}
	return ledger, nil
}

// Add txset and save it to db and cache.
func (lm *Manager) AddTxSet(txsetHash string, txset *ultpb.TxSet) error {
	tb, err := ultpb.Encode(txset)
	if err != nil {
		return fmt.Errorf("encode txset failed: %v", err)
	}

	// Save the txset in db first.
	err = lm.database.Put(lm.txsetBucket, []byte(txsetHash), tb)
	if err != nil {
		return fmt.Errorf("save tx list to db failed: %v", err)
	}

	lm.txsetCache.Add(txsetHash, txset)

	return nil
}

// Get the txset of the corresponding txset hash.
func (lm *Manager) GetTxSet(txsetHash string) (*ultpb.TxSet, error) {
	if txs, ok := lm.txsetCache.Get(txsetHash); ok {
		return txs.(*ultpb.TxSet), nil
	}

	txb, err := lm.database.Get(lm.txsetBucket, []byte(txsetHash))
	if err != nil {
		return nil, fmt.Errorf("get txset from db failed: %v", err)
	}
	if txb == nil {
		return nil, nil
	}

	txset, err := ultpb.DecodeTxSet(txb)
	if err != nil {
		return nil, fmt.Errorf("decode txset failed: %v", err)
	}

	// Cache the txset.
	lm.txsetCache.Add(txsetHash, txset)

	return txset, nil
}

// Receive externalized consensus value and do appropriate operations
// depend on current state of the ledger.
func (lm *Manager) RecvExtVal(index uint64, value string, txset *ultpb.TxSet) error {
	log.Infow("recv ext value", "seq", index, "value", value, "prevhash", txset.PrevLedgerHash, "txcount", len(txset.TxList))

	switch lm.ledgerState {
	case LedgerStateNotSynced:
		fallthrough
	case LedgerStateSynced:
		if lm.NextLedgerHeaderSeq() == index { // normal case
			if lm.CurrLedgerHeaderHash() == txset.PrevLedgerHash {
				err := lm.closeLedger(index, value, txset)
				if err != nil {
					return fmt.Errorf("close ledger failed: %v", err)
				}
			} else {
				log.Fatalw("ledger hash inconsistent", "prevHash", txset.PrevLedgerHash, "currHash", lm.CurrLedgerHeaderHash())
			}
			log.Infow("ledger closed successfully", "prevHash", lm.prevLedgerHeaderHash, "currHash", lm.currLedgerHeaderHash, "nextSeq", lm.NextLedgerHeaderSeq())
			lm.largestConsensusIndex = index
			lm.ledgerState = LedgerStateSynced
		} else if index < lm.NextLedgerHeaderSeq() { // old case
			log.Warnw("recv an old consensus value", "nextSeq", lm.NextLedgerHeaderSeq())
		} else { // new case
			log.Warnw("ledger out of sync", "nextSeq", lm.NextLedgerHeaderSeq(), "extSeq", index)
			lm.buffer.Append(&CloseInfo{Index: index, Value: value, TxSet: txset})
			if index <= lm.largestConsensusIndex {
				// We came across an older unprocessed consensus value
				// which should be on the way of downloading.
				return nil
			}
			// Start to download missing ledgers.
			err := lm.downloader.AddTask(lm.largestConsensusIndex+1, index-1)
			if err != nil {
				return fmt.Errorf("add task to downloader failed: %v", err)
			}
			lm.largestConsensusIndex = index
			lm.ledgerState = LedgerStateSyncing
		}
	case LedgerStateSyncing:
		// Append to buffer until local ledger catches up.
		lm.buffer.Append(&CloseInfo{Index: index, Value: value, TxSet: txset})
	}

	return nil
}

// Recover the latest ledger states from checkpoint.
func (lm *Manager) RecoverFromCheckpoint() error {
	b, err := lm.database.Get(lm.bucket, []byte("CHECKPOINT"))
	if err != nil {
		return fmt.Errorf("get ledger checkpoint failed: %v", err)
	}

	// Set the ledger to the state of NotSynced as we do not
	// know the current states of the consensus yet.
	lm.ledgerState = LedgerStateNotSynced

	// Ledger checkpoint could be empty for a joining node.
	if b == nil {
		return nil
	}
	checkpoint, err := ultpb.DecodeLedgerCheckpoint(b)
	if err != nil {
		return fmt.Errorf("decode ledger checkpoint failed: %v", err)
	}

	// Do sanity checks.
	now := time.Now().Unix()
	if checkpoint.LastCloseTime < 0 || checkpoint.LastCloseTime > now {
		return errors.New("invalid LastCloseTime")
	}
	if checkpoint.LedgerHeaderCount < 0 {
		return errors.New("invalid LedgerHeaderCount")
	}
	if checkpoint.LargestConsensusIndex < 0 {
		return errors.New("invalid LargestConsensus")
	}
	if checkpoint.PrevLedgerHeader == nil && checkpoint.LedgerHeaderCount != 1 {
		return errors.New("PrevLedgerHeader is missing")
	}
	if checkpoint.LargestConsensusIndex > 1 {
		prevHash, err := ultpb.GetLedgerHeaderKey(checkpoint.PrevLedgerHeader)
		if err != nil {
			return fmt.Errorf("get prev ledger header key failed: %v", err)
		}
		if prevHash != checkpoint.PrevLedgerHeaderHash {
			return errors.New("incompatible prev ledger hash")
		}
	}
	currHash, err := ultpb.GetLedgerHeaderKey(checkpoint.CurrLedgerHeader)
	if err != nil {
		return fmt.Errorf("get curr ledger header key failed: %v", err)
	}
	if currHash != checkpoint.CurrLedgerHeaderHash {
		return errors.New("incompatible curr ledger hash")
	}

	// The previous hash of current ledger should be the same as
	// the PrevLedgerHeaderHash.
	if checkpoint.CurrLedgerHeader.PrevLedgerHash != checkpoint.PrevLedgerHeaderHash {
		return errors.New("incompatible prev hash of current ledger")
	}

	// Recover the states of the ledger.
	lm.lastCloseTime = checkpoint.LastCloseTime
	lm.ledgerHeaderCount = checkpoint.LedgerHeaderCount
	lm.largestConsensusIndex = checkpoint.LargestConsensusIndex
	lm.prevLedgerHeader = checkpoint.PrevLedgerHeader
	lm.prevLedgerHeaderHash = checkpoint.PrevLedgerHeaderHash
	lm.currLedgerHeader = checkpoint.CurrLedgerHeader
	lm.currLedgerHeaderHash = checkpoint.CurrLedgerHeaderHash

	return nil
}

// Save the checkpoint of the current states of the ledger.
func (lm *Manager) saveCheckpoint() error {
	ckpt := &ultpb.LedgerCheckpoint{
		LastCloseTime:         lm.lastCloseTime,
		LedgerHeaderCount:     lm.ledgerHeaderCount,
		LargestConsensusIndex: lm.largestConsensusIndex,
		PrevLedgerHeader:      lm.prevLedgerHeader,
		PrevLedgerHeaderHash:  lm.prevLedgerHeaderHash,
		CurrLedgerHeader:      lm.currLedgerHeader,
		CurrLedgerHeaderHash:  lm.currLedgerHeaderHash,
	}

	b, err := ultpb.Encode(ckpt)
	if err != nil {
		return fmt.Errorf("encode checkpoint failed: %v", err)
	}

	err = lm.database.Put(lm.bucket, []byte("CHECKPOINT"), b)
	if err != nil {
		return fmt.Errorf("save checkpoint in db failed: %v", err)
	}

	return nil
}

// CloseLedger closes current ledger with new consensus value.
func (lm *Manager) closeLedger(index uint64, value string, txset *ultpb.TxSet) error {
	if lm.ledgerState != LedgerStateSynced {
		return errors.New("ledger is not synced")
	}

	// Check whether the sequence number is valid.
	if lm.currLedgerHeader != nil && lm.currLedgerHeader.SeqNum+1 != index {
		return fmt.Errorf("ledger seqnum mismatch: expect %d, got %d", lm.currLedgerHeader.SeqNum+1, index)
	}

	// Check whether the supplied prev ledger header hash is identical
	// to the current ledger header hash. It should never happen that
	// these two values are not identical which means some fork happened.
	if lm.currLedgerHeader != nil && lm.currLedgerHeaderHash != txset.PrevLedgerHash {
		log.Fatalw("ledger tx header hash mismatch", "curr", lm.currLedgerHeaderHash, "prev", txset.PrevLedgerHash)
	}

	txsetHash, err := ultpb.GetTxSetKey(txset)
	if err != nil {
		return fmt.Errorf("get txset hash failed: %v", err)
	}

	b, err := b58.Decode(value)
	if err != nil {
		return fmt.Errorf("hex decode consensus value failed: %v", err)
	}
	cv, err := ultpb.DecodeConsensusValue(b)
	if err != nil {
		return fmt.Errorf("decode consensus value failed: %v", err)
	}

	if cv.TxSetHash != txsetHash {
		return fmt.Errorf("txset hash mismatch: cv txset hash %s, computed txset hash %s", cv.TxSetHash, txsetHash)
	}

	// Run transactions for identifying any protential errors.
	err = lm.tm.ApplyTxList(txset.TxList, lm.NextLedgerHeaderSeq())
	if err != nil {
		return fmt.Errorf("apply tx list failed: %v", err)
	}

	err = lm.advanceLedger(index, txset.PrevLedgerHash, txsetHash, value)
	if err != nil {
		log.Fatalf("advance ledger failed: %v", err)
	}

	return nil
}

// Append the committed transaction list to current ledger, the operation
// is only valid when the ledger manager in synced state.
func (lm *Manager) advanceLedger(seq uint64, prevHeaderHash string, txHash string, cv string) error {
	header := &ultpb.LedgerHeader{
		SeqNum:         seq,
		PrevLedgerHash: prevHeaderHash,
		TxSetHash:      txHash,
		ConsensusValue: cv,
		Version:        GenesisVersion,
		MaxTxListSize:  GenesisMaxTxSetSize,
		TotalTokens:    GenesisTotalTokens,
		BaseFee:        GenesisBaseFee,
		BaseReserve:    GenesisBaseReserve,
	}
	b, err := ultpb.Encode(header)
	if err != nil {
		return fmt.Errorf("encode ledger header failed: %v", err)
	}

	// Encode ledger header to ULTKey.
	h, err := ultpb.GetLedgerHeaderKey(header)
	if err != nil {
		return fmt.Errorf("encode ledger header to key failed: %v", err)
	}

	// Save the ledger header.
	seqStr := strconv.FormatUint(header.SeqNum, 10)
	err = lm.database.Put(lm.bucket, []byte(seqStr), b)
	if err != nil {
		fmt.Errorf("save ledger in db failed: %v", err)
	}
	lm.headers.Add(seqStr, header)

	// Advance current ledger header.
	lm.prevLedgerHeader = lm.currLedgerHeader
	lm.prevLedgerHeaderHash = lm.currLedgerHeaderHash
	lm.currLedgerHeader = header
	lm.currLedgerHeaderHash = h

	lm.ledgerHeaderCount += 1
	lm.lastCloseTime = time.Now().Unix()

	// Save the ledger checkpoint.
	err = lm.saveCheckpoint()
	if err != nil {
		return fmt.Errorf("save ledger checkpoint failed: %v", err)
	}

	return nil
}
