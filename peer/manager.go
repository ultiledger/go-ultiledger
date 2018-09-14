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

// Manager manages the CRUD of peers
type Manager struct {
	// Network address of the node
	addr string

	// NodeID of the node
	nodeID string

	// Metadata for gRPC context
	metadata metadata.MD

	// max number of peers to connect
	maxPeers int

	// initial peer addresses
	initPeers []string

	// unhealthy peer addresses
	unhealthyPeers []string

	// connected peers
	peerLock  sync.RWMutex
	livePeers map[string]*Peer

	nodeIDs mapset.Set

	// peers waiting to be connected, each peer has three
	// chances to be connected.
	pendingPeers map[string]int

	// channel for stopping pending peers connection
	stopChan chan struct{}

	// channel for adding peer addr
	peerAddrChan chan string

	// channel for managing live peers
	addChan    chan *Peer
	deleteChan chan *Peer
}

func NewManager(ps []string, addr string, nodeID string) *Manager {
	return &Manager{
		addr:         addr,
		nodeID:       nodeID,
		metadata:     metadata.Pairs(addr, nodeID),
		maxPeers:     100, // hard code for now
		initPeers:    ps,
		pendingPeers: make(map[string]int),
		livePeers:    make(map[string]*Peer),
		nodeIDs:      mapset.NewSet(),
		stopChan:     make(chan struct{}),
		peerAddrChan: make(chan string, 100),
		addChan:      make(chan *Peer),
		deleteChan:   make(chan *Peer),
	}
}

func (pm *Manager) Start() {
	go func() {
		// connect to inital peers
		for _, addr := range pm.initPeers {
			p, err := pm.connectPeer(addr)
			if err != nil {
				log.Errorf("connect to peer %s failed: %v", addr, err)
				pm.pendingPeers[addr] = 3
				continue
			}
			pm.livePeers[addr] = p
		}

		go pm.connect()

		for {
			select {
			case p := <-pm.addChan:
				pm.peerLock.Lock()
				pm.livePeers[p.Addr] = p
				pm.nodeIDs.Add(p.NodeID)
				pm.peerLock.Unlock()
			case p := <-pm.deleteChan: // only delete connected peers
				pm.peerLock.Lock()
				if _, ok := pm.livePeers[p.Addr]; ok {
					delete(pm.livePeers, p.Addr)
					pm.nodeIDs.Remove(p.NodeID)
				}
				pm.peerLock.Unlock()
			}
		}
	}()
}

// Stop the peer manager
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

// Get a list of rpc clients from live peers
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

// Add new peer with network addr
func (pm *Manager) AddPeerAddr(addr string) error {
	select {
	case pm.peerAddrChan <- addr:
	case <-pm.stopChan:
		return errors.New("peer manager stopped")
	}
	return nil
}

// connects the remote peer with provided network address
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

// connect to peers periodically
func (pm *Manager) connect() {
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			if len(pm.pendingPeers) == 0 {
				continue
			}
			if len(pm.livePeers) > pm.maxPeers {
				continue
			}
			for addr, count := range pm.pendingPeers {
				if count == 0 {
					delete(pm.pendingPeers, addr)
					continue
				}
				p, err := pm.connectPeer(addr)
				if err != nil {
					log.Errorf("connect to peer %s failed: %v", addr, err)
					count -= 1
					pm.pendingPeers[addr] = count
					continue
				}
				// healthcheck the peer and save the nodeID
				remoteAddr, nodeID, err := rpc.Hello(p.client, p.metadata)
				if err != nil {
					log.Errorf("say hello to peer %s failed: %v", addr, err)
					// save unhealthy peers for later reconnect
					pm.unhealthyPeers = append(pm.unhealthyPeers, addr)
					p.Close()
					continue
				}
				p.Addr = remoteAddr // TODO(bobonovski) check whether the dial IP is the same as the response IP?
				p.NodeID = nodeID

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
			pm.pendingPeers[addr] = 3
		case <-pm.stopChan:
			return
		}
	}
}
