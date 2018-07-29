package server

import (
	"golang.org/x/net/context"

	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type ULTNodeServer struct{}

func (s *ULTNodeServer) HealthCheck(ctx context.Context, req *rpc.HealthCheckRequest) (*rpc.HealthCheckResponse, error) {
	return nil, nil
}
