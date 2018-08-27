package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type NodeServer struct {
	ip     string // IP address of this node
	nodeID string // ID of this node (public key)
	seed   string // Private key of this node

	nodeKey map[string]*crypto.ULTKey

	// channel for adding new peer IP
	peerChan chan string
	// channel for adding new tx
	txChan chan *ultpb.Tx
}

func NewNodeServer(ip string, nodeID string, seed string, peerC chan string, txC chan *ultpb.Tx) *NodeServer {
	s := &NodeServer{ip: ip, nodeID: nodeID, seed: seed, peerChan: peerC, txChan: txC}
	return s
}

func (s *NodeServer) Hello(ctx context.Context, req *rpcpb.HelloRequest) (*rpcpb.HelloResponse, error) {
	resp := &rpcpb.HelloResponse{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, errors.New("failed to retrieve incoming context")
	}
	if len(md.Get("IP")) == 0 || len(md.Get("NodeID")) == 0 {
		return resp, errors.New("IP address or NodeID is absent")
	}
	s.peerChan <- md.Get("IP")[0]
	k, err := crypto.DecodeKey(md.Get("NodeID")[0])
	if err != nil {
		return resp, errors.New("invalid node ID")
	}
	s.nodeKey[md.Get("IP")[0]] = k
	grpc.SendHeader(ctx, metadata.Pairs("IP", s.ip, "NodeID", s.nodeID))
	return resp, nil
}

func (s *NodeServer) SubmitTx(ctx context.Context, req *rpcpb.SubmitTxRequest) (*rpcpb.SubmitTxResponse, error) {
	return nil, nil
}

func (s *NodeServer) Nominate(ctx context.Context, req *rpcpb.NominateRequest) (*rpcpb.NominateResponse, error) {
	return nil, nil
}
