package grpc

import (
	"fmt"
	"time"

	g "google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Hub manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type GrpcClient struct {
	networkID     string
	accountID     string
	coreEndpoints string
	client        rpcpb.NodeClient
}

func New(networkID, accoountID, coreEndpoints string) (*GrpcClient, error) {
	// connect to core servers
	r := NewResolver()
	b := g.RoundRobin(r)
	conn, err := g.Dial(coreEndpoints, g.WithInsecure(), g.WithBalancer(b), g.WithBlock(), g.WithTimeout(time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to core servers failed: %v", err)
	}
	client := rpcpb.NewNodeClient(conn)
	gc := &GrpcClient{
		networkID:     networkID,
		accountID:     accountID,
		coreEndpoints: coreEndpoints,
		client:        client,
	}
	return &gc, nil
}

// SummitTx summits the tx to ult servers and return appropriate
// response messages to client.
func (c *GrpcClient) SummitTx() {
	log.Info("got summit tx request")
}

// QueryTx query the tx status from ult servers and return current
// tx status.
func (c *GrpcClient) QueryTx() {
	log.Info("got query tx request")
}
