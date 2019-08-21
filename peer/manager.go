// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package peer

import (
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

	// Channel for managing live peers.
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
		deleteChan:   make(chan *Peer),
	}
}

func (pm *Manager) Start() {
	go pm.connect()

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
	for _, p := range pm.livePeers {
		p.Close()
	}
	pm.livePeers = nil
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

// Get the metadata of the node.
func (pm *Manager) GetMetadata() metadata.MD {
	return pm.metadata
}

// Add new peer with network addr.
func (pm *Manager) AddPeerAddr(addr string) {
	if _, ok := pm.pendingPeers[addr]; ok {
		return
	}
	if _, ok := pm.livePeers[addr]; ok {
		return
	}
	pm.pendingPeers[addr] = struct{}{}
	return
}

// Connects the remote peer with provided network address.
func (pm *Manager) connectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(time.Second), grpc.WithBackoffMaxDelay(100*time.Millisecond))
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
	heartbeatTicker := time.NewTicker(time.Second)
	reconnectTicker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-heartbeatTicker.C:
			// Say hello to connected peers for maintaining the
			// node identity when the peer is down.
			for _, p := range pm.livePeers {
				remoteAddr, nodeID, err := rpc.Hello(p.client, p.metadata, pm.networkID)
				if err != nil {
					continue
				}
				p.Addr = remoteAddr
				p.NodeID = nodeID
			}
		case <-reconnectTicker.C:
			if len(pm.pendingPeers) == 0 {
				continue
			}
			if len(pm.livePeers) > pm.maxPeers {
				continue
			}
			var connected []string
			for addr, _ := range pm.pendingPeers {
				log.Debugw("retry to connect to peer", "addr", addr)
				p, err := pm.connectPeer(addr)
				if err != nil {
					log.Errorf("connect to peer %s failed: %v", addr, err)
					continue
				}
				log.Infow("connected to peer", "addr", p.Addr)

				pm.peerLock.Lock()
				pm.livePeers[p.Addr] = p
				pm.nodeIDs.Add(p.NodeID)
				pm.peerLock.Unlock()

				connected = append(connected, addr)

				if len(pm.livePeers) > pm.maxPeers {
					break
				}
			}
			for _, addr := range connected {
				delete(pm.pendingPeers, addr)
			}
		case p := <-pm.deleteChan:
			if _, ok := pm.livePeers[p.Addr]; ok {
				delete(pm.livePeers, p.Addr)
				pm.nodeIDs.Remove(p.NodeID)
			}
		case <-pm.stopChan:
			return
		}
	}
}
