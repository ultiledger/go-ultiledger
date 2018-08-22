package api

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/consensus"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/peer"
	"github.com/ultiledger/go-ultiledger/ultpb/rpc"
)

type NodeServer struct {
	IP     string // ip address of this node
	NodeID string // ID of this node (public key)

	nodeKey map[string]*crypto.ULTKey

	pm     *peer.Manager
	engine *consensus.Engine
}

func NewNodeServer(ip string, nodeID string, pm *peer.Manager, eng *consensus.Engine) *NodeServer {
	s := &NodeServer{IP: ip, NodeID: nodeID, pm: pm, engine: eng}
	return s
}

func (s *NodeServer) Hello(ctx context.Context, req *rpc.HelloRequest) (*rpc.HelloResponse, error) {
	resp := &rpc.HelloResponse{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, errors.New("failed to retrieve incoming context")
	}
	if len(md.Get("IP")) == 0 || len(md.Get("NodeID")) == 0 {
		return resp, errors.New("IP address or NodeID is absent")
	}
	if err := s.pm.AddPeer(md.Get("IP")[0]); err != nil {
		return resp, err
	}
	k, err := crypto.DecodeKey(md.Get("NodeID")[0])
	if err != nil {
		return resp, errors.New("invalid node ID")
	}
	s.nodeKey[md.Get("IP")[0]] = k
	grpc.SendHeader(ctx, metadata.Pairs("IP", s.IP, "NodeID", s.NodeID))
	return resp, nil
}

func (s *NodeServer) SubmitTx(ctx context.Context, req *rpc.SubmitTxRequest) (*rpc.SubmitTxResponse, error) {
	return nil, nil
}
