package peer

import (
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

// Peer represents the overall information about the remote peer
type Peer struct {
	// peer network address (ip:port)
	Addr string
	// the role of the peer
	Role string
	// connection time
	ConnTime int64

	// grpc service client
	client rpc.ULTNodeClient
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
