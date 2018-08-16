package peer

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"

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
func (p *Peer) HealthCheck(ip string, nodeID string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(1*time.Second))
	defer cancel()

	req := rpc.HealthCheckRequest{IP: ip, NodeID: nodeID}
	resp, err := p.client.HealthCheck(ctx, &req)
	if err != nil {
		return "", "", err
	}
	if resp.IP == "" || resp.NodeID == "" {
		return "", "", errors.New("empty peer IP or NodeID")
	}

	return resp.IP, resp.NodeID, nil
}
