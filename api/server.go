package api

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/crypto"
	pb "github.com/ultiledger/go-ultiledger/ultpb"
	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type NodeServer struct {
	IP     string // ip address of this node
	NodeID string // ID of this node (public key)

	nodeKey map[string]*crypto.ULTKey

	peerChan chan string // channel for adding new peer IP

	txChan       chan *pb.Tx        // channel for transaction submission
	nominateChan chan *pb.Statement // channel for nomination statement
}

func NewNodeServer(ip string, nodeID string, txC chan *pb.Tx, nominateC chan *pb.Statement) *NodeServer {
	s := &NodeServer{IP: ip, txChan: txC, nominateChan: nominateC}
	return s
}

func (s *NodeServer) Hello(ctx context.Context, req *rpc.HelloRequest) (*rpc.HelloResponse, error) {
	resp := &rpc.HelloResponse{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, errors.New("failed to retrieve incoming context")
	}
	if len(md.Get("IP")) > 0 && len(md.Get("NodeID")) > 0 {
		s.peerChan <- md.Get("IP")[0]
		k, err := crypto.DecodeKey(md.Get("NodeID")[0])
		if err != nil {
			return resp, errors.New("invalid node ID")
		}
		s.nodeKey[md.Get("IP")[0]] = k
	}
	grpc.SendHeader(ctx, metadata.Pairs("IP", s.IP, "NodeID", s.NodeID))
	return resp, nil
}

func (s *NodeServer) SubmitTx(ctx context.Context, req *rpc.SubmitTxRequest) (*rpc.SubmitTxResponse, error) {
	return nil, nil
}
