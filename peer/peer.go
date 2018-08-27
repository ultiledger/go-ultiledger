package peer

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Peer represents the overall information about the remote peer
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
