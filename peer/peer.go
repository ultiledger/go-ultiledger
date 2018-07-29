package peer

import (
	"time"

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
	Client rpc.ULTNodeClient
	// underlying network connection
	Conn *grpc.ClientConn
}

// The string representation of the peer is its ip:port address.
func (p Peer) String() string {
	return p.Addr
}

// ConnectPeer connects the remote peer with provided network address
func ConnectPeer(addr string) (*Peer, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		return nil, err
	}
	client := rpc.NewULTNodeClient(conn)
	return &Peer{Addr: addr, ConnTime: time.Now().Unix(), Client: client, Conn: conn}, nil
}
