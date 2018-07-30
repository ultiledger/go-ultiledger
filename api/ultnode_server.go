package api

import (
	"golang.org/x/net/context"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type ULTNodeServer struct{}

func (s *ULTNodeServer) HealthCheck(ctx context.Context, req *rpc.HealthCheckRequest) (*rpc.HealthCheckResponse, error) {
	resp := &rpc.HealthCheckResponse{}
	return resp, nil
}

func (s *ULTNodeServer) SubmitTransaction(ctx context.Context, req *rpc.SubmitTransactionRequest) (*rpc.SubmitTransactionResponse, error) {
	return nil, nil
}
