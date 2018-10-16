package service

import (
	"fmt"
	"time"

	"github.com/emicklei/go-restful"
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Hub manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type Hub struct {
	coreEndpoints string
	client        rpcpb.NodeClient
}

func NewHub(coreEndpoints string) (*Hub, error) {
	// connect to core servers
	r := NewResolver()
	b := grpc.RoundRobin(r)
	conn, err := grpc.Dial(coreEndpoints, grpc.WithInsecure(), grpc.WithBalancer(b), grpc.WithBlock(), grpc.WithTimeout(time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to core servers failed: %v", err)
	}
	client := rpcpb.NewNodeClient(conn)
	return &Hub{coreEndpoints: coreEndpoints, client: client}, nil
}

// SummitTx summits the tx to ult servers and return appropriate
// response messages to client.
func (h *Hub) SummitTx(request *restful.Request, response *restful.Response) {
	log.Info("got summit tx request")
}

// QueryTx query the tx status from ult servers and return current
// tx status.
func (h *Hub) QueryTx(request *restful.Request, response *restful.Response) {
	log.Info("got query tx request")
}
