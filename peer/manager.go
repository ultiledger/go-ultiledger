package peer

import (
	"errors"
	"sync"
	"time"

	"github.com/deckarep/golang-set"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Manager manages known remote peers.
type Manager struct {
	// Network id of the network.
	networkID string

	// Network address of the node.
	addr string

	// NodeID of the node.
	nodeID string

	// Metadata for gRPC context.
	metadata metadata.MD

	// Max number of peers to connect.
	maxPeers int

	// Initial peer addresses.
	initPeers []string

	// Unhealthy peer addresses.
	unhealthyPeers []string

	// Connected peers.
	peerLock  sync.RWMutex
	livePeers map[string]*Peer

	nodeIDs mapset.Set

	// Peers waiting to be connected.
	pendingPeers map[string]struct{}

	// Channel for stopping pending peers connection.
	stopChan chan struct{}

	// Channel for adding peer addr.
	peerAddrChan chan string

	// Channel for managing live peers.
	addChan    chan *Peer
	deleteChan chan *Peer
}

func NewManager(ps []string, networkID string, addr string, nodeID string, maxPeers int) *Manager {
	return &Manager{
		networkID:    networkID,
		addr:         addr,
		nodeID:       nodeID,
		metadata:     metadata.Pairs("addr", addr, "nodeid", nodeID),
		maxPeers:     maxPeers,
		initPeers:    ps,
		pendingPeers: make(map[string]struct{}),
		livePeers:    make(map[string]*Peer),
		nodeIDs:      mapset.NewSet(),
		stopChan:     make(chan struct{}),
		peerAddrChan: make(chan string, 100),
		addChan:      make(chan *Peer),
		deleteChan:   make(chan *Peer),
	}
}

func (pm *Manager) Start() {
	go pm.connect()

	go func() {
		for {
			select {
			case p := <-pm.addChan:
				pm.peerLock.Lock()
				pm.livePeers[p.Addr] = p
				pm.nodeIDs.Add(p.NodeID)
				pm.peerLock.Unlock()
			case p := <-pm.deleteChan: // Only delete connected peers.
				pm.peerLock.Lock()
				if _, ok := pm.livePeers[p.Addr]; ok {
					delete(pm.livePeers, p.Addr)
					pm.nodeIDs.Remove(p.NodeID)
				}
				pm.peerLock.Unlock()
			case <-pm.stopChan:
				break
			}
		}
	}()

	go func() {
		// Connect to inital peers.
		for _, addr := range pm.initPeers {
			p, err := pm.connectPeer(addr)
			if err != nil {
				log.Errorf("connect to peer %s failed: %v", addr, err)
				pm.pendingPeers[addr] = struct{}{}
				continue
			}
			log.Infof("connected to peer %s", addr)
			pm.livePeers[addr] = p
		}
	}()
}

// Stop the peer manager.
func (pm *Manager) Stop() {
	close(pm.stopChan)
	pm.pendingPeers = nil
	pm.peerLock.Lock()
	for _, p := range pm.livePeers {
		p.Close()
	}
	pm.livePeers = nil
	pm.peerLock.Unlock()
}

// Get a list of rpc clients from live peers.
func (pm *Manager) GetLiveClients() []rpcpb.NodeClient {
	pm.peerLock.RLock()
	defer pm.peerLock.RUnlock()
	var clients []rpcpb.NodeClient
	for _, p := range pm.livePeers {
		clients = append(clients, p.client)
	}
	return clients
}

func (pm *Manager) GetMetadata() metadata.MD {
	return pm.metadata
}

// Add new peer with network addr.
func (pm *Manager) AddPeerAddr(addr string) error {
	select {
	case pm.peerAddrChan <- addr:
	case <-pm.stopChan:
		return errors.New("peer manager stopped")
	}
	return nil
}

// Connects the remote peer with provided network address.
func (pm *Manager) connectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(time.Second))
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
	// Healthcheck the peer and save the node id.
	remoteAddr, nodeID, err := rpc.Hello(p.client, p.metadata, pm.networkID)
	if err != nil {
		p.Close()
		return nil, err
	}
	p.Addr = remoteAddr
	p.NodeID = nodeID
	return p, nil
}

// Connect to new peers periodically.
func (pm *Manager) connect() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			if len(pm.pendingPeers) == 0 {
				continue
			}
			if len(pm.livePeers) > pm.maxPeers {
				continue
			}
			for addr, _ := range pm.pendingPeers {
				p, err := pm.connectPeer(addr)
				if err != nil {
					log.Errorf("connect to peer %s failed: %v", addr, err)
					continue
				}
				log.Infow("connected to peer", "addr", p.Addr)

				delete(pm.pendingPeers, addr)

				select {
				case pm.addChan <- p:
				case <-pm.stopChan:
					return
				}

				if len(pm.livePeers) > pm.maxPeers {
					break
				}
			}
		case addr := <-pm.peerAddrChan:
			if _, ok := pm.pendingPeers[addr]; ok {
				continue
			}
			if _, ok := pm.livePeers[addr]; ok {
				continue
			}
			pm.pendingPeers[addr] = struct{}{}
		case <-pm.stopChan:
			return
		}
	}
}
