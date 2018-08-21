package peer

import (
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

// PeerManager manages the CRUD of peers
type PeerManager struct {
	logger *zap.SugaredLogger

	// IP of the node
	IP string

	// NodeID of the node
	NodeID string

	// Metadata for gRPC context
	metadata metadata.MD

	// initial peer addresses
	initPeers []string

	// peers waiting for connection
	waitPeers []string

	// connected peers
	livePeers map[string]*Peer

	// peers need to retry to connect other peers, at most three
	// times. If the left retry count is zero, it means this peer
	// is bad and temporarily no need to try to connect it again.
	// If in some situation, we get request from this peer then the
	// retry will start again.
	retryPeers    map[string]int
	retryStopChan chan struct{}
	retryChan     chan string

	// channel for managing peers
	addChan    chan *Peer
	deleteChan chan *Peer
}

func NewPeerManager(l *zap.SugaredLogger, ps []string, ip string, nodeID string) *PeerManager {
	return &PeerManager{
		logger:        l,
		IP:            ip,
		NodeID:        nodeID,
		metadata:      metadata.Pairs(ip, nodeID),
		initPeers:     ps,
		retryPeers:    make(map[string]int),
		livePeers:     make(map[string]*Peer),
		retryStopChan: make(chan struct{}),
	}
}

func (pm *PeerManager) Start(stopChan chan struct{}) {
	go func() {
		// connect to inital peers
		for _, addr := range pm.initPeers {
			p, err := pm.connectPeer(addr)
			if err != nil {
				pm.logger.Warnw("failed to connect to peer", "addr", addr)
				pm.retryPeers[addr] = 3
				continue
			}
			pm.livePeers[addr] = p
		}

		go pm.retryConnect()

		for {
			select {
			case p := <-pm.addChan:
				pm.livePeers[p.Addr] = p
			case p := <-pm.deleteChan: // only delete connected peers
				if _, ok := pm.livePeers[p.Addr]; ok {
					delete(pm.livePeers, p.Addr)
				}
			case <-stopChan:
				close(pm.retryStopChan) // stop retry first
				pm.retryPeers = nil
				for _, p := range pm.livePeers {
					p.Close()
				}
				pm.livePeers = nil
				return
			}
		}
	}()
}

// ConnectPeer connects the remote peer with provided network address
func (pm *PeerManager) connectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(1*time.Second))
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
			if len(pm.retryPeers) == 0 {
				continue
			}
			for addr, count := range pm.retryPeers {
				if count == 0 {
					delete(pm.retryPeers, addr)
					continue
				}
				p, err := pm.connectPeer(addr)
				if err != nil {
					pm.logger.Warnw("failed to connect to peer", "addr", addr)
					count -= 1
					pm.retryPeers[addr] = count
					continue
				}
				// healthcheck the peer and save the nodeID
				ip, nodeID, err := p.HealthCheck(pm.IP, pm.NodeID)
				if err != nil {
					pm.logger.Warnw("peer is not health", "peerIP", p.Addr)
					p.Close()
					continue
				}
				p.Addr = ip // TODO(bobonovski) check whether the dial IP is the same as the response IP?
				p.NodeID = nodeID

				delete(pm.retryPeers, addr)

				select {
				case pm.addChan <- p:
				case <-pm.retryStopChan:
					return
				}
			}
		case addr := <-pm.retryChan:
			if _, ok := pm.retryPeers[addr]; ok {
				continue
			}
			pm.retryPeers[addr] = 3
		case <-pm.retryStopChan:
			return
		}
	}
}
