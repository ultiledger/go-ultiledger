package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/future"
	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// NodeServer creates a gRPC server to accept requests from peers,
// it does not contain any handlers of internal components, all the
// requests are processed by passing futures to internal Node which
// controls all the internal components to generate corresponding
// responses and errors
type NodeServer struct {
	addr   string // Network address of this node
	nodeID string // ID of this node (public key)
	seed   string // Private key of this node

	// network address to nodeID map
	nodeKey map[string]*crypto.ULTKey

	// future for adding peer addr
	peerFuture chan<- *future.Peer
	// future for adding tx
	txFuture chan<- *future.Tx
	// future for adding consensus statement
	stmtFuture chan<- *future.Statement

	// future for query quorum
	quorumFuture chan<- *future.Quorum
	// future for query txset
	txsetFuture chan<- *future.TxSet
	// future for query tx status
	txsFuture chan<- *future.TxStatus
}

// ServerContext represents contextual information for running server
type ServerContext struct {
	Addr           string                 // local network address
	NodeID         string                 // local node ID
	Seed           string                 // local node seed
	PeerFuture     chan *future.Peer      // channel for sending discovered peer to node
	TxFuture       chan *future.Tx        // channel for sending received tx to node
	StmtFuture     chan *future.Statement // channel for sending received statement to node
	QuorumFuture   chan *future.Quorum    // channel for sending quorum query to consensus engine
	TxSetFuture    chan *future.TxSet     // channel for sending txset query to consensus engine
	TxStatusFuture chan *future.TxStatus  // channel for sending txstatus query to consensus engine
}

func ValidateServerContext(sc *ServerContext) error {
	if sc == nil {
		return errors.New("server context is nil")
	}
	if sc.Addr == "" {
		return errors.New("empty local network address")
	}
	if sc.NodeID == "" {
		return errors.New("empty local node ID")
	}
	if sc.Seed == "" {
		return errors.New("empty local node seed")
	}
	if sc.PeerFuture == nil {
		return errors.New("peer future channel is nil")
	}
	if sc.TxFuture == nil {
		return errors.New("tx future channel is nil")
	}
	if sc.StmtFuture == nil {
		return errors.New("statement future channel is nil")
	}
	if sc.QuorumFuture == nil {
		return errors.New("quorum future channel is nil")
	}
	if sc.TxSetFuture == nil {
		return errors.New("txset future channel is nil")
	}
	return nil
}

// NewNodeServer creates a NodeServer instance with server context
func NewNodeServer(ctx *ServerContext) *NodeServer {
	if err := ValidateServerContext(ctx); err != nil {
		log.Fatalf("validate server context failed: %v", err)
	}
	server := &NodeServer{
		addr:         ctx.Addr,
		nodeID:       ctx.NodeID,
		seed:         ctx.Seed,
		peerFuture:   ctx.PeerFuture,
		txFuture:     ctx.TxFuture,
		stmtFuture:   ctx.StmtFuture,
		quorumFuture: ctx.QuorumFuture,
		txsetFuture:  ctx.TxSetFuture,
	}
	return server
}

// Validate checks the ip and nodeID info from metadata and
// checks the correctness of digital signature of the data
func (s *NodeServer) validate(ctx context.Context, data []byte, signature string) error {
	// retrieve nodeID and ip addr
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("retrieve incoming context failed")
	}
	if len(md.Get("Addr")) == 0 || len(md.Get("NodeID")) == 0 {
		return errors.New("network address or nodeID is absent")
	}

	// check whether we know this addr
	addr := md.Get("Addr")[0]
	if _, ok := s.nodeKey[addr]; !ok {
		return fmt.Errorf("unknown network address %s, forgot to say hello?", addr)
	}
	key := s.nodeKey[addr]

	// check signature
	if !crypto.VerifyByKey(key, signature, data) {
		return errors.New("signature verification failed")
	}

	return nil
}

// Hello retrieves network address and nodeID from context and
// respond with network address and nodeID of local node
func (s *NodeServer) Hello(ctx context.Context, req *rpcpb.HelloRequest) (*rpcpb.HelloResponse, error) {
	resp := &rpcpb.HelloResponse{}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, errors.New("retrieve incoming context failed")
	}
	if len(md.Get("Addr")) == 0 || len(md.Get("NodeID")) == 0 {
		return resp, errors.New("network address or nodeID is absent")
	}

	// validate nodeID
	k, err := crypto.DecodeKey(md.Get("NodeID")[0])
	if err != nil {
		return resp, fmt.Errorf("decode nodeID to crypto key failed: %v", err)
	}
	if k.Code != crypto.KeyTypeNodeID {
		return resp, errors.New("invalid nodeID key type")
	}
	s.nodeKey[md.Get("Addr")[0]] = k

	// add peer address
	f := &future.Peer{Addr: md.Get("Addr")[0]}
	f.Init()
	s.peerFuture <- f
	if err := f.Error(); err != nil { // just log error message
		log.Error(err)
	}

	grpc.SendHeader(ctx, metadata.Pairs("Addr", s.addr, "NodeID", s.nodeID))

	return resp, nil
}

func (s *NodeServer) SubmitTx(ctx context.Context, req *rpcpb.SubmitTxRequest) (*rpcpb.SubmitTxResponse, error) {
	resp := &rpcpb.SubmitTxResponse{}

	// decode pb to tx
	tx, err := ultpb.DecodeTx(req.Data)
	if err != nil {
		return resp, err
	}

	// get account key
	accKey, err := crypto.DecodeKey(tx.AccountID)
	if err != nil {
		return resp, err
	}
	if accKey.Code != crypto.KeyTypeAccountID {
		return resp, errors.New("invalid account ID")
	}

	// verify signature
	if !crypto.VerifyByKey(accKey, req.Signature, req.Data) {
		return resp, errors.New("invalid signature")
	}

	// verify tx key
	txKey, err := crypto.DecodeKey(req.TxKey)
	if err != nil {
		return resp, errors.New("decode tx key failed")
	}
	txHash, err := ultpb.SHA256HashBytes(tx)
	if err != nil {
		return resp, errors.New("compute tx hash failed")
	}
	if !bytes.Equal(txHash[:], txKey.Hash[:]) {
		return resp, errors.New("tx key hash mismatch")
	}

	f := &future.Tx{Tx: tx, TxKey: req.TxKey}
	f.Init()
	s.txFuture <- f
	if err := f.Error(); err != nil {
		return resp, err
	}
	return resp, nil
}

// Query accepts information query requests from peers and
// return encoded information if the node has it
func (s *NodeServer) Query(ctx context.Context, req *rpcpb.QueryRequest) (*rpcpb.QueryResponse, error) {
	resp := &rpcpb.QueryResponse{}

	if err := s.validate(ctx, req.Data, req.Signature); err != nil {
		return resp, fmt.Errorf("input validation faile: %v", err)
	}

	switch req.MsgType {
	case rpcpb.QueryMsgType_QUORUM:
		qf := &future.Quorum{QuorumHash: string(req.Data)}
		qf.Init()
		s.quorumFuture <- qf
		if err := qf.Error(); err != nil {
			return resp, fmt.Errorf("query quorum failed: %v", err)
		}
		qb, err := ultpb.Encode(qf.Quorum)
		if err != nil {
			return resp, fmt.Errorf("encode quorum failed: %v", err)
		}
		resp.Data = qb
	case rpcpb.QueryMsgType_TXSET:
		txf := &future.TxSet{TxSetHash: string(req.Data)}
		txf.Init()
		s.txsetFuture <- txf
		if err := txf.Error(); err != nil {
			return resp, fmt.Errorf("query txset failed: %v", err)
		}
		txb, err := ultpb.Encode(txf.TxSet)
		if err != nil {
			return resp, fmt.Errorf("encode txset failed: %v", err)
		}
		resp.Data = txb
	}

	return resp, nil
}

// Notify accepts transaction and consensus message and
// redistribute message to internal managing components
func (s *NodeServer) Notify(ctx context.Context, req *rpcpb.NotifyRequest) (*rpcpb.NotifyResponse, error) {
	resp := &rpcpb.NotifyResponse{}

	if err := s.validate(ctx, req.Data, req.Signature); err != nil {
		return resp, fmt.Errorf("input validation faile: %v", err)
	}

	switch req.MsgType {
	case rpcpb.NotifyMsgType_TX:
		tx, err := ultpb.DecodeTx(req.Data)
		if err != nil {
			return resp, fmt.Errorf("decode tx failed: %v", err)
		}
		txf := &future.Tx{Tx: tx}
		txf.Init()
		s.txFuture <- txf
		if err := txf.Error(); err != nil {
			return resp, fmt.Errorf("add tx failed: %v", err)
		}
	case rpcpb.NotifyMsgType_STATEMENT:
		stmt, err := ultpb.DecodeStatement(req.Data)
		if err != nil {
			return resp, fmt.Errorf("decode tx failed: %v", err)
		}
		sf := &future.Statement{Stmt: stmt}
		sf.Init()
		s.stmtFuture <- sf
		if err := sf.Error(); err != nil {
			return resp, fmt.Errorf("decode statement failed: %v", err)
		}
	}

	return resp, nil
}
