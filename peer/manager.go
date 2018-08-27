package peer

import (
	"errors"
	"time"

	"github.com/deckarep/golang-set"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

var (
	ErrInvalidIP = errors.New("invalid IP address format")
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
	nodeIDs   mapset.Set

	// connect channel for add new peer IP
	ConnectChan chan string

	// peers waiting to be connected, each peer has three
	// chances to be connected.
	pendingPeers map[string]int

	// channel for stopping pending peers connection
	stopChan chan struct{}

	// channel for managing live peers
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
		nodeIDs:      mapset.NewSet(),
		ConnectChan:  make(chan string, 100),
		stopChan:     make(chan struct{}),
		addChan:      make(chan *Peer),
		deleteChan:   make(chan *Peer),
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
				pm.nodeIDs.Add(p.NodeID)
			case p := <-pm.deleteChan: // only delete connected peers
				if _, ok := pm.livePeers[p.Addr]; ok {
					delete(pm.livePeers, p.Addr)
					pm.nodeIDs.Remove(p.NodeID)
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

// Get the snapshot of current live nodeIDs
func (pm *Manager) GetLiveNodeID() mapset.Set {
	return pm.nodeIDs.Clone()
}

// ConnectPeer connects the remote peer with provided network address
func (pm *Manager) connectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(1*time.Second))
	if err != nil {
		return nil, err
	}
	client := rpcpb.NewNodeClient(conn)
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
				ip, nodeID, err := rpc.Hello(p.client, p.metadata)
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
		case addr := <-pm.ConnectChan:
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
