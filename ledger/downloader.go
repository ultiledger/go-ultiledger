package ledger

import (
	"errors"
	"strconv"
	"sync"

	b58 "github.com/mr-tron/base58/base58"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/db"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type DownloadRange struct {
	StartIndex uint64
	EndIndex   uint64
}

// Downloader downloads missing ledgers from peers.
type Downloader struct {
	database db.Database
	bucket   string

	// peer manager
	pm *peer.Manager

	// seed of local node
	seed string

	// next index waiting for processing by ledger manager
	nextIndex uint64

	// ledger index to close info map
	infoMap map[uint64]*CloseInfo

	// channel for dispatching download task
	rangeChan chan *DownloadRange

	// channel for notifying ready CloseInfo
	readyChan chan *CloseInfo

	// channel for reorder received CloseInfo
	reorderChan chan *CloseInfo

	// channel for stopping goroutines
	stopChan chan struct{}
}

// Create a new instance of downloader.
func NewDownloader(db db.Database, pm *peer.Manager) *Downloader {
	dlr := &Downloader{
		database:  db,
		bucket:    "DOWNLOADER",
		pm:        pm,
		nextIndex: uint64(0),
		infoMap:   make(map[uint64]*CloseInfo),
		rangeChan: make(chan *DownloadRange, 1),
		readyChan: make(chan *CloseInfo),
		stopChan:  make(chan struct{}),
	}

	return dlr
}

// Add download task with start index and end index.
func (d *Downloader) AddTask(start uint64, end uint64) error {
	if start > end {
		return errors.New("invalid ledger index range")
	}

	log.Infow("received ledger download task", "start", start, "end", end)

	d.rangeChan <- &DownloadRange{StartIndex: start, EndIndex: end}

	return nil
}

// Start the downloader.
func (d *Downloader) Start() {
	go d.run()
	go d.reorder()
}

// Stop the downloader by notifying goroutines to stop.
func (d *Downloader) Stop() {
	close(d.stopChan)
	d.infoMap = nil
}

// Ready returns downloaded ledgers from start index to end index.
func (d *Downloader) Ready() <-chan *CloseInfo {
	return d.readyChan
}

// Event loop for handling download task.
func (d *Downloader) run() {
	for {
		select {
		case tr := <-d.rangeChan:
			d.nextIndex = tr.StartIndex
			err := d.download(tr)
			if err != nil {
				log.Errorf("download task failed: %v", err, "start", tr.StartIndex, "end", tr.EndIndex)
			}
		case <-d.stopChan:
			return
		}
	}
}

// Reorder received CloseInfo in expected return order.
func (d *Downloader) reorder() {
	for {
		select {
		case info := <-d.reorderChan:
			d.infoMap[info.Index] = info
			for {
				ci, ok := d.infoMap[d.nextIndex]
				if !ok {
					break
				}
				d.readyChan <- ci
				d.nextIndex += 1
			}
		case <-d.stopChan:
			return
		}
	}
}

// Download ledgers concurrently by asking each peer one by one
// about the ledger index.
func (d *Downloader) download(tr *DownloadRange) error {
	done := make(chan bool)

	tasks := d.prepareTask(done, tr)

	workers := make([]<-chan *CloseInfo, 10) // hard code for now
	for i := 0; i < 10; i++ {
		workers[i] = d.runTask(done, tasks)
	}

	for info := range d.mergeInfo(done, workers...) {
		d.reorderChan <- info
	}

	close(done)

	return nil
}

// Prepare ledger download task for concurrent processing.
func (d *Downloader) prepareTask(done <-chan bool, tr *DownloadRange) <-chan uint64 {
	taskChan := make(chan uint64)

	go func() {
		for i := tr.StartIndex; i <= tr.EndIndex; i++ {
			taskChan <- i
		}
	}()

	return taskChan
}

// Run ledger download task by query peers one by one.
func (d *Downloader) runTask(done <-chan bool, taskChan <-chan uint64) <-chan *CloseInfo {
	infoChan := make(chan *CloseInfo)

	query := func(i uint64) *CloseInfo {
		clients := d.pm.GetLiveClients()
		metadata := d.pm.GetMetadata()

		// encode the ledger index
		seq := strconv.FormatUint(i, 10)
		payload := []byte(seq)

		// sign the data
		sign, err := crypto.Sign(d.seed, payload)
		if err != nil {
			log.Errorf("sign payload for ledger %d query failed: %v", i, err)
			return nil
		}

		// query ledger from peers
		ledger, err := rpc.QueryLedger(clients, metadata, payload, sign)
		if err != nil {
			log.Errorf("rpc query ledger %d failed: %v", i, err)
			return nil
		}

		// validate received ledger
		header := ledger.LedgerHeader
		txset := ledger.TxSet

		txsetHash, err := ultpb.SHA256Hash(txset)
		if err != nil {
			log.Errorf("compute txset hash failed: %v", err)
			return nil
		}

		if header.TxSetHash != txsetHash {
			log.Errorw("header txset hash incompatible with txsetHash", "headerTxSetHash", header.TxSetHash, "txsetHash", txsetHash)
			return nil
		}

		// check consensus value
		cvb, err := b58.Decode(header.ConsensusValue)
		if err != nil {
			log.Errorf("hex decode consensus value failed: %v", err)
			return nil
		}
		cv, err := ultpb.DecodeConsensusValue(cvb)
		if err != nil {
			log.Errorf("decode consensus value failed: %v", err)
			return nil
		}

		if cv.TxSetHash != txsetHash {
			log.Errorw("cv txset hash incompatible with txsetHash", "cvTxSetHash", cv.TxSetHash, "txsetHash", txsetHash)
			return nil
		}

		info := &CloseInfo{Index: i, Value: header.ConsensusValue, TxSet: txset}

		return info
	}

	go func() {
		for t := range taskChan {
			select {
			case <-done:
				return
			case infoChan <- query(t):
			}
		}
	}()

	return infoChan
}

// Merge info from multiple workers to return a merged response channel
func (d *Downloader) mergeInfo(done <-chan bool, infoChans ...<-chan *CloseInfo) <-chan *CloseInfo {
	var wg sync.WaitGroup
	wg.Add(len(infoChans))

	result := make(chan *CloseInfo)
	multiplex := func(infoChan <-chan *CloseInfo) {
		defer wg.Done()
		for info := range infoChan {
			if info == nil {
				continue
			}
			select {
			case <-done:
				return
			case result <- info:
			}
		}
	}

	for _, c := range infoChans {
		go multiplex(c)
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}
