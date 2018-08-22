package peer

import (
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

// Manager manages the CRUD of peers
type Manager struct {
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

	// peers waiting to be connected, each peer has three
	// chances to be connected.
	pendingPeers map[string]int
	connectChan  chan string

	// channel for stopping pending peers connection
	stopChan chan struct{}

	// channel for managing peers
	addChan    chan *Peer
	deleteChan chan *Peer
}

func NewManager(l *zap.SugaredLogger, ps []string, ip string, nodeID string) *Manager {
	return &Manager{
		logger:       l,
		IP:           ip,
		NodeID:       nodeID,
		metadata:     metadata.Pairs(ip, nodeID),
		initPeers:    ps,
		pendingPeers: make(map[string]int),
		livePeers:    make(map[string]*Peer),
		stopChan:     make(chan struct{}),
	}
}

func (pm *Manager) Start(stopChan chan struct{}) {
	go func() {
		// connect to inital peers
		for _, addr := range pm.initPeers {
			p, err := pm.connectPeer(addr)
			if err != nil {
				pm.logger.Warnw("failed to connect to peer", "addr", addr)
				pm.pendingPeers[addr] = 3
				continue
			}
			pm.livePeers[addr] = p
		}

		go pm.connect()

		for {
			select {
			case p := <-pm.addChan:
				pm.livePeers[p.Addr] = p
			case p := <-pm.deleteChan: // only delete connected peers
				if _, ok := pm.livePeers[p.Addr]; ok {
					delete(pm.livePeers, p.Addr)
				}
			case <-stopChan:
				close(pm.stopChan) // stop retry first
				pm.pendingPeers = nil
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
func (pm *Manager) connectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(1*time.Second))
	if err != nil {
		return nil, err
	}
	client := rpc.NewNodeClient(conn)
	p := &Peer{
		Addr:     addr,
		ConnTime: time.Now().Unix(),
		metadata: pm.metadata,
		client:   client,
		conn:     conn,
	}
	return p, nil
}

// try to connect to peers periodically
func (pm *Manager) connect() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			if len(pm.pendingPeers) == 0 {
				continue
			}
			for addr, count := range pm.pendingPeers {
				if count == 0 {
					delete(pm.pendingPeers, addr)
					continue
				}
				p, err := pm.connectPeer(addr)
				if err != nil {
					pm.logger.Warnw("failed to connect to peer", "addr", addr)
					count -= 1
					pm.pendingPeers[addr] = count
					continue
				}
				// healthcheck the peer and save the nodeID
				ip, nodeID, err := p.Hello()
				if err != nil {
					pm.logger.Warnw("peer is not health", "peerIP", p.Addr)
					p.Close()
					continue
				}
				p.Addr = ip // TODO(bobonovski) check whether the dial IP is the same as the response IP?
				p.NodeID = nodeID

				delete(pm.pendingPeers, addr)

				select {
				case pm.addChan <- p:
				case <-pm.stopChan:
					return
				}
			}
		case addr := <-pm.connectChan:
			if _, ok := pm.pendingPeers[addr]; ok {
				continue
			}
			if _, ok := pm.livePeers[addr]; ok {
				continue
			}
			pm.pendingPeers[addr] = 3
		case <-pm.stopChan:
			return
		}
	}
}
