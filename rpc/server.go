package rpc

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/future"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type NodeServer struct {
	logger *zap.SugaredLogger
	addr   string // Network address of this node
	nodeID string // ID of this node (public key)
	seed   string // Private key of this node

	nodeKey map[string]*crypto.ULTKey

	// future for adding new peer addr
	peerFuture chan *future.Peer
	// future for adding new tx
	txFuture chan *future.Tx
}

func NewNodeServer(l *zap.SugaredLogger, addr string, nodeID string, seed string, peerC chan *future.Peer, txC chan *future.Tx) *NodeServer {
	s := &NodeServer{logger: l, addr: addr, nodeID: nodeID, seed: seed, peerFuture: peerC, txFuture: txC}
	return s
}

func (s *NodeServer) Hello(ctx context.Context, req *rpcpb.HelloRequest) (*rpcpb.HelloResponse, error) {
	resp := &rpcpb.HelloResponse{}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, errors.New("failed to retrieve incoming context")
	}
	if len(md.Get("Addr")) == 0 || len(md.Get("NodeID")) == 0 {
		return resp, errors.New("Network address or NodeID is absent")
	}
	f := &future.Peer{Addr: md.Get("Addr")[0]}
	f.Init()
	s.peerFuture <- f
	if err := f.Error(); err != nil {
		s.logger.Warnf("failed to add new peer: %v", err)
	}
	k, err := crypto.DecodeKey(md.Get("NodeID")[0])
	if err != nil {
		return resp, errors.New("invalid node ID")
	}
	s.nodeKey[md.Get("Addr")[0]] = k
	grpc.SendHeader(ctx, metadata.Pairs("Addr", s.addr, "NodeID", s.nodeID))
	return resp, nil
}

func (s *NodeServer) SubmitTx(ctx context.Context, req *rpcpb.SubmitTxRequest) (*rpcpb.SubmitTxResponse, error) {
	resp := &rpcpb.SubmitTxResponse{}
	tx, err := ultpb.DecodeTx(req.Data)
	if err != nil {
		return resp, err
	}
	f := &future.Tx{Tx: tx}
	f.Init()
	s.txFuture <- f
	if err := f.Error(); err != nil {
		return resp, err
	}
	return nil, nil
}

func (s *NodeServer) Notify(ctx context.Context, req *rpcpb.NotifyRequest) (*rpcpb.NotifyResponse, error) {
	return nil, nil
}