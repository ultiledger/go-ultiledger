package peer

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
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

// HealthCheck checks the health of remote peer and at the
// same time exchanges nodeID (public key) between peers
func (p *Peer) HealthCheck() (string, string, error) {
	ctx := metadata.NewOutgoingContext(context.Background(), p.metadata)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Second))
	defer cancel()

	var header metadata.MD

	req := rpc.HealthCheckRequest{}
	_, err := p.client.HealthCheck(ctx, &req, grpc.Header(&header))
	if err != nil {
		return "", "", err
	}
	if len(header.Get("IP")) == 0 || len(header.Get("NodeID")) == 0 {
		return "", "", errors.New("empty peer IP or NodeID")
	}

	return header.Get("IP")[0], header.Get("NodeID")[0], nil
}
