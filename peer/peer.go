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
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Peer represents the overall information about the remote peer.
type Peer struct {
	// peer network address (ip:port)
	Addr string
	// NodeID of the peer (public key)
	NodeID string
	// the role of the peer
	Role string
	// connection time
	ConnTime int64
	// metadata for outgoing context
	metadata metadata.MD
	// grpc service client
	client rpcpb.NodeClient
	// underlying network connection
	conn *grpc.ClientConn
}

// The string representation of the peer is its ip:port address.
func (p Peer) String() string {
	return p.Addr
}

// close the underlying connection
func (p *Peer) Close() {
	if p.conn != nil {
		p.conn.Close()
	}
}
