package api

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type ULTNodeServer struct {
	IP           string             // ip address of this node
	NodeID       string             // ID of this node (public key)
	peerChan     chan string        // channel for adding new peer IP
	txChan       chan *pb.Tx        // channel for transaction submission
	nominateChan chan *pb.Statement // channel for nomination statement
}

func NewULTNodeServer(ip string, nodeID string, txC chan *pb.Tx, nominateC chan *pb.Statement) *ULTNodeServer {
	s := &ULTNodeServer{IP: ip, txChan: txC, nominateChan: nominateC}
	return s
}

func (s *ULTNodeServer) HealthCheck(ctx context.Context, req *rpc.HealthCheckRequest) (*rpc.HealthCheckResponse, error) {
	resp := &rpc.HealthCheckResponse{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, errors.New("failed to retrieve incoming context")
	}
	if len(md.Get("IP")) > 0 {
		s.peerChan <- md.Get("IP")[0]
	}
	grpc.SendHeader(ctx, metadata.Pairs("IP", s.IP, "NodeID", s.NodeID))
	return resp, nil
}

func (s *ULTNodeServer) SubmitTransaction(ctx context.Context, req *rpc.SubmitTransactionRequest) (*rpc.SubmitTransactionResponse, error) {
	return nil, nil
}
