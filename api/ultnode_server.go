package api

import (
	"golang.org/x/net/context"

	pb "github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type ULTNodeServer struct {
	IP     string               // ip address of this node
	txChan chan *pb.Transaction // transaction submittion channel
}

func NewULTNodeServer(ip string) *ULTNodeServer {
	s := &ULTNodeServer{IP: ip, txChan: make(chan *pb.Transaction)}
	return s
}

func (s *ULTNodeServer) HealthCheck(ctx context.Context, req *rpc.HealthCheckRequest) (*rpc.HealthCheckResponse, error) {
	resp := &rpc.HealthCheckResponse{}
	return resp, nil
}

func (s *ULTNodeServer) SubmitTransaction(ctx context.Context, req *rpc.SubmitTransactionRequest) (*rpc.SubmitTransactionResponse, error) {
	return nil, nil
}
