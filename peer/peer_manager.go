package peer

import (
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

// PeerManager manages the CRUD of peers
type PeerManager struct {
	logger *zap.SugaredLogger
	// initial peer addresses
	initPeers []string

	// peers waiting for connection
	waitPeers []string

	// connected peers
	livePeers map[string]*Peer
	liveLock  sync.RWMutex

	// peers need to retry to connect other peers, at most three
	// times. If the left retry count is zero, it means this peer
	// is bad and temporarily no need to try to connect it again.
	// If in some situation, we get request from this peer then the
	// retry will start again.
	retryPeers    map[string]int
	retryLock     sync.Mutex
	retryStopChan chan struct{}

	// channel for managing peers
	addChan    chan string
	deleteChan chan string
}

func NewPeerManager(l *zap.SugaredLogger, ps []string) *PeerManager {
	return &PeerManager{initPeers: ps}
}

func (pm *PeerManager) Start(stopChan chan struct{}) {
	// connect to inital peers
	for _, addr := range pm.initPeers {
		p, err := pm.connectPeer(addr)
		if err != nil {
			pm.logger.Errorw("failed to connect to peer", "addr", addr)
			pm.retryPeers[addr] = 3 // retry three times and no need lock here
			continue
		}
		pm.livePeers[addr] = p // no need lock here
	}

	go pm.retryConnect()

	for {
		select {
		case addr := <-pm.addChan:
			p, err := pm.connectPeer(addr)
			if err != nil {
				pm.logger.Errorw("failed to connect to peer", "addr", addr)
				pm.retryLock.Lock()
				pm.retryPeers[addr] = 3
				pm.retryLock.Unlock()
				continue
			}
			pm.liveLock.Lock()
			pm.livePeers[addr] = p
			pm.liveLock.Unlock()
		case addr := <-pm.deleteChan: // only delete connected peers
			pm.liveLock.Lock()
			if _, ok := pm.livePeers[addr]; ok {
				delete(pm.livePeers, addr)
			}
			pm.liveLock.Unlock()
			continue
		case <-stopChan:
			pm.retryStopChan <- struct{}{} // stop retry first
			pm.retryPeers = nil
			pm.liveLock.Lock()
			for _, p := range pm.livePeers {
				p.Close()
			}
			pm.livePeers = nil
			pm.liveLock.Unlock()
			return
		}
	}
}

// ConnectPeer connects the remote peer with provided network address
func (pm *PeerManager) connectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		return nil, err
	}
	client := rpc.NewULTNodeClient(conn)
	return &Peer{Addr: addr, ConnTime: time.Now().Unix(), client: client, conn: conn}, nil
}

// retry to connect peer
func (pm *PeerManager) retryConnect() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			if len(pm.retryPeers) > 0 {
				pm.retryLock.Lock()
				for addr, count := range pm.retryPeers {
					if count == 0 {
						continue
					}
					p, err := pm.connectPeer(addr)
					if err != nil {
						count -= 1
						pm.retryPeers[addr] = count
						continue
					}
					pm.liveLock.Lock()
					pm.livePeers[addr] = p
					pm.liveLock.Unlock()
				}
				pm.retryLock.Unlock()
			}
		case <-pm.retryStopChan:
			return
		}
	}
}
