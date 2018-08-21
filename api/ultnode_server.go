package api

import (
	"golang.org/x/net/context"

	pb "github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type ULTNodeServer struct {
	IP           string             // ip address of this node
	NodeID       string             // ID of this node (public key)
	txChan       chan *pb.Tx        // transaction submission channel
	nominateChan chan *pb.Statement // nomination statement channel
}

func NewULTNodeServer(ip string, nodeID string, txC chan *pb.Tx, nominateC chan *pb.Statement) *ULTNodeServer {
	s := &ULTNodeServer{IP: ip, txChan: txC, nominateChan: nominateC}
	return s
}

func (s *ULTNodeServer) HealthCheck(ctx context.Context, req *rpc.HealthCheckRequest) (*rpc.HealthCheckResponse, error) {
	resp := &rpc.HealthCheckResponse{
		IP:     s.IP,
		NodeID: s.NodeID,
	}
	return resp, nil
}

func (s *ULTNodeServer) SubmitTransaction(ctx context.Context, req *rpc.SubmitTransactionRequest) (*rpc.SubmitTransactionResponse, error) {
	return nil, nil
}
