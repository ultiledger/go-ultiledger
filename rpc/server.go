// Copyright 2019 The go-ultiledger Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ultiledger/go-ultiledger/account"
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
// responses and errors.
type NodeServer struct {
	networkID string // Hash of the networkID.
	addr      string // Network address of this node.
	nodeID    string // ID of this node (public key).
	seed      string // Private key of this node.

	am *account.Manager

	// Peer network address to nodeID map.
	nodeKey sync.Map

	// Future for adding peer addr.
	peerFuture chan<- *future.Peer
	// Future for adding tx.
	txFuture chan<- *future.Tx
	// Future for adding consensus statement.
	stmtFuture chan<- *future.Statement

	// Future for querying ledger.
	ledgerFuture chan<- *future.Ledger
	// Future for querying quorum.
	quorumFuture chan<- *future.Quorum
	// Future for querying txset.
	txsetFuture chan<- *future.TxSet
	// Future for querying tx status.
	txsFuture chan<- *future.TxStatus
	// Future for querying accounts.
	accountFuture chan<- *future.Account
}

// ServerContext represents contextual information for running server.
type ServerContext struct {
	NetworkID      string
	Addr           string
	NodeID         string
	Seed           string
	AM             *account.Manager
	PeerFuture     chan *future.Peer
	TxFuture       chan *future.Tx
	StmtFuture     chan *future.Statement
	LedgerFuture   chan *future.Ledger
	QuorumFuture   chan *future.Quorum
	TxSetFuture    chan *future.TxSet
	TxStatusFuture chan *future.TxStatus
	AccountFuture  chan *future.Account
}

func ValidateServerContext(sc *ServerContext) error {
	if sc == nil {
		return errors.New("server context is nil")
	}
	if sc.NetworkID == "" {
		return errors.New("empty network ID")
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
	if sc.AM == nil {
		return errors.New("account manager is nil")
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
	if sc.LedgerFuture == nil {
		return errors.New("ledger future channel is nil")
	}
	if sc.QuorumFuture == nil {
		return errors.New("quorum future channel is nil")
	}
	if sc.TxSetFuture == nil {
		return errors.New("txset future channel is nil")
	}
	if sc.TxStatusFuture == nil {
		return errors.New("txstatus future channel is nil")
	}
	if sc.AccountFuture == nil {
		return errors.New("account future channel is nil")
	}
	return nil
}

// NewNodeServer creates a NodeServer instance with server context.
func NewNodeServer(ctx *ServerContext) *NodeServer {
	if err := ValidateServerContext(ctx); err != nil {
		log.Fatalf("validate server context failed: %v", err)
	}
	server := &NodeServer{
		networkID:     ctx.NetworkID,
		addr:          ctx.Addr,
		nodeID:        ctx.NodeID,
		seed:          ctx.Seed,
		am:            ctx.AM,
		peerFuture:    ctx.PeerFuture,
		txFuture:      ctx.TxFuture,
		stmtFuture:    ctx.StmtFuture,
		ledgerFuture:  ctx.LedgerFuture,
		quorumFuture:  ctx.QuorumFuture,
		txsetFuture:   ctx.TxSetFuture,
		txsFuture:     ctx.TxStatusFuture,
		accountFuture: ctx.AccountFuture,
	}
	return server
}

// Validate checks the ip and nodeID info from metadata and
// checks the correctness of digital signature of the data.
func (s *NodeServer) validate(ctx context.Context, data []byte, signature string, networkID string) error {
	if s.networkID != networkID {
		return errors.New("incompatible network id.")
	}

	// Retrieve nodeID and ip addr.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("retrieve incoming context failed")
	}
	if len(md.Get("Addr")) == 0 || len(md.Get("NodeID")) == 0 {
		return errors.New("network address or nodeID is absent")
	}

	// Check whether we know this addr.
	addr := md.Get("Addr")[0]
	k, ok := s.nodeKey.Load(addr)
	if !ok {
		return fmt.Errorf("unknown network address %s, forgot to say hello?", addr)
	}
	key := k.(*crypto.ULTKey)

	// Check signature.
	if !crypto.VerifyByKey(key, signature, data) {
		return errors.New("signature verification failed")
	}

	return nil
}

// Hello retrieves network address and nodeid from context and
// respond with network address and nodeid of local node.
func (s *NodeServer) Hello(ctx context.Context, req *rpcpb.HelloRequest) (*rpcpb.HelloResponse, error) {
	resp := &rpcpb.HelloResponse{}

	if s.networkID != req.NetworkID {
		return resp, status.Error(codes.InvalidArgument, "incompatible network id.")
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return resp, status.Error(codes.NotFound, "retrieve incoming context failed")
	}
	if len(md.Get("addr")) == 0 || len(md.Get("nodeid")) == 0 {
		return resp, status.Error(codes.NotFound, "network address or nodeid is missing")
	}

	// Validate node id of the peer.
	k, err := crypto.DecodeKey(md.Get("nodeid")[0])
	if err != nil {
		return resp, status.Error(codes.InvalidArgument, "decode nodeid to crypto key failed")
	}
	if k.Code != crypto.KeyTypeNodeID {
		return resp, status.Error(codes.InvalidArgument, "invalid nodeid key type")
	}
	s.nodeKey.Store(md.Get("addr")[0], k)

	// Add the peer address.
	f := &future.Peer{Addr: md.Get("addr")[0]}
	f.Init()
	s.peerFuture <- f
	if err := f.Error(); err != nil {
		log.Errorf("add peer to peer manager failed: %v", err)
	}

	grpc.SendHeader(ctx, metadata.Pairs("addr", s.addr, "nodeid", s.nodeID))

	return resp, nil
}

func (s *NodeServer) SubmitTx(ctx context.Context, req *rpcpb.SubmitTxRequest) (*rpcpb.SubmitTxResponse, error) {
	resp := &rpcpb.SubmitTxResponse{}

	log.Infow("received new tx submission", "req", req)

	if s.networkID != req.NetworkID {
		return resp, status.Error(codes.InvalidArgument, "incompatible network id.")
	}

	// Decode pb to Tx.
	tx, err := ultpb.DecodeTx(req.Data)
	if err != nil {
		return resp, status.Error(codes.InvalidArgument, "decode tx failed")
	}

	// Get the account key.
	accKey, err := crypto.DecodeKey(tx.AccountID)
	if err != nil {
		return resp, status.Error(codes.InvalidArgument, "decode account key failed")
	}
	if accKey.Code != crypto.KeyTypeAccountID {
		return resp, status.Error(codes.InvalidArgument, "invalid account key type")
	}

	// Verify signature.
	if !crypto.VerifyByKey(accKey, req.Signature, req.Data) {
		return resp, status.Error(codes.InvalidArgument, "signature verification failed")
	}

	// Verify tx key.
	txKey, err := crypto.DecodeKey(req.TxKey)
	if err != nil {
		return resp, status.Error(codes.InvalidArgument, "decode tx key failed")
	}
	txHash, err := ultpb.SHA256HashBytes(tx)
	if err != nil {
		return resp, status.Error(codes.Internal, "compute tx hash failed")
	}
	if !bytes.Equal(txHash[:], txKey.Hash[:]) {
		return resp, status.Error(codes.Internal, "tx key hash mismatch")
	}

	f := &future.Tx{Tx: tx, TxKey: req.TxKey}
	f.Init()
	s.txFuture <- f
	if err := f.Error(); err != nil {
		return resp, status.Errorf(codes.Internal, "submit tx failed: %v", err)
	}
	return resp, nil
}

// QueryTx queries the status of the transaction.
func (s *NodeServer) QueryTx(ctx context.Context, req *rpcpb.QueryTxRequest) (*rpcpb.QueryTxResponse, error) {
	resp := &rpcpb.QueryTxResponse{}

	log.Infow("received new tx query", "txKey", req.TxKey)

	if s.networkID != req.NetworkID {
		return resp, status.Error(codes.InvalidArgument, "incompatible network id.")
	}

	// Check the validity of the tx key.
	if !crypto.IsValidTxKey(req.TxKey) {
		return resp, status.Errorf(codes.InvalidArgument, "invalid tx key")
	}

	f := &future.TxStatus{TxKey: req.TxKey}
	f.Init()
	s.txsFuture <- f
	if err := f.Error(); err != nil {
		return resp, status.Errorf(codes.Internal, "query tx status failed: %v", err)
	}

	resp.TxStatus = f.TxStatus

	return resp, nil
}

// GetAccount queries the account information.
func (s *NodeServer) GetAccount(ctx context.Context, req *rpcpb.GetAccountRequest) (*rpcpb.GetAccountResponse, error) {
	resp := &rpcpb.GetAccountResponse{}

	if s.networkID != req.NetworkID {
		return resp, status.Error(codes.InvalidArgument, "incompatible network id.")
	}

	// Check the validity of the account key.
	if !crypto.IsValidAccountKey(req.AccountID) {
		return resp, status.Errorf(codes.InvalidArgument, "invalid account id")
	}

	f := &future.Account{AccountID: req.AccountID}
	f.Init()
	s.accountFuture <- f
	if err := f.Error(); err != nil {
		return resp, status.Errorf(codes.Internal, "get account failed: %v", err)
	}

	b, err := ultpb.Encode(f.Account)
	if err != nil {
		return resp, status.Error(codes.Internal, "encode account failed")
	}
	resp.Data = b

	return resp, nil
}

// CreateTestAccount creates a test account.
func (s *NodeServer) CreateTestAccount(ctx context.Context, req *rpcpb.CreateTestAccountRequest) (*rpcpb.CreateTestAccountResponse, error) {
	resp := &rpcpb.CreateTestAccountResponse{}

	if s.networkID != req.NetworkID {
		return resp, status.Error(codes.InvalidArgument, "incompatible network id.")
	}

	// Check the validity of the account key.
	if !crypto.IsValidAccountKey(req.AccountID) {
		return resp, status.Errorf(codes.InvalidArgument, "invalid account id")
	}

	// Check whether the account exists.
	f := &future.Account{AccountID: req.AccountID}
	f.Init()
	s.accountFuture <- f
	if err := f.Error(); err != nil {
		return resp, status.Errorf(codes.Internal, "get account failed: %v", err)
	}
	if f.Account != nil {
		return resp, status.Errorf(codes.Internal, "test account already exist")
	}

	tx := &ultpb.Tx{
		AccountID: s.am.Master.AccountID,
		Fee:       int64(1000),
		SeqNum:    s.am.Master.SeqNum,
	}
	tx.OpList = append(tx.OpList, &ultpb.Op{
		OpType: ultpb.OpType_CREATE_ACCOUNT,
		Op: &ultpb.Op_CreateAccount{
			&ultpb.CreateAccountOp{
				AccountID: req.AccountID,
				Balance:   int64(100000000000), // 10 ULT
			},
		},
	})

	// Get the tx key.
	txKey, err := ultpb.GetTxKey(tx)
	if err != nil {
		return resp, status.Errorf(codes.Internal, "get tx key failed: %v", err)
	}

	txf := &future.Tx{Tx: tx, TxKey: txKey}
	txf.Init()
	s.txFuture <- txf
	if err := txf.Error(); err != nil {
		return resp, status.Errorf(codes.Internal, "submit tx failed: %v", err)
	}
	resp.TxKey = txKey

	return resp, nil
}

// Query accepts information query requests from peers and
// return encoded information if the node has it.
func (s *NodeServer) Query(ctx context.Context, req *rpcpb.QueryRequest) (*rpcpb.QueryResponse, error) {
	resp := &rpcpb.QueryResponse{}

	if err := s.validate(ctx, req.Data, req.Signature, req.NetworkID); err != nil {
		return resp, status.Errorf(codes.InvalidArgument, "input validation failed: %v", err)
	}

	switch req.MsgType {
	case rpcpb.QueryMsgType_QUORUM:
		qf := &future.Quorum{QuorumHash: string(req.Data)}
		qf.Init()
		s.quorumFuture <- qf
		if err := qf.Error(); err != nil {
			return resp, status.Errorf(codes.Internal, "query quorum failed: %v", err)
		}
		if qf.Quorum == nil {
			return resp, status.Error(codes.NotFound, "quorum not found")
		}
		qb, err := ultpb.Encode(qf.Quorum)
		if err != nil {
			return resp, status.Error(codes.Internal, "encode quorum failed")
		}
		resp.Data = qb
	case rpcpb.QueryMsgType_TXSET:
		txf := &future.TxSet{TxSetHash: string(req.Data)}
		txf.Init()
		s.txsetFuture <- txf
		if err := txf.Error(); err != nil {
			return resp, status.Errorf(codes.Internal, "query txset failed: %v", err)
		}
		if txf.TxSet == nil {
			return resp, status.Error(codes.NotFound, "txset not found")
		}
		txb, err := ultpb.Encode(txf.TxSet)
		if err != nil {
			return resp, status.Error(codes.Internal, "encode txset failed")
		}
		resp.Data = txb
	case rpcpb.QueryMsgType_LEDGER:
		lf := &future.Ledger{LedgerSeq: string(req.Data)}
		lf.Init()
		s.ledgerFuture <- lf
		if err := lf.Error(); err != nil {
			return resp, status.Errorf(codes.Internal, "query ledger failed: %v", err)
		}
		if lf.Ledger == nil {
			return resp, status.Error(codes.NotFound, "ledger not found")
		}
		lb, err := ultpb.Encode(lf.Ledger)
		if err != nil {
			return resp, status.Error(codes.Internal, "encode ledger failed")
		}
		resp.Data = lb
	}

	return resp, nil
}

// Notify accepts transaction and consensus message and
// redistribute message to internal managing components.
func (s *NodeServer) Notify(ctx context.Context, req *rpcpb.NotifyRequest) (*rpcpb.NotifyResponse, error) {
	resp := &rpcpb.NotifyResponse{}

	if err := s.validate(ctx, req.Data, req.Signature, req.NetworkID); err != nil {
		return resp, status.Errorf(codes.InvalidArgument, "input validation failed: %v", err)
	}

	switch req.MsgType {
	case rpcpb.NotifyMsgType_TX:
		tx, err := ultpb.DecodeTx(req.Data)
		if err != nil {
			return resp, status.Error(codes.InvalidArgument, "decode tx failed")
		}
		txKey, err := ultpb.GetTxKey(tx)
		if err != nil {
			return resp, status.Errorf(codes.Internal, "get tx key failed: %v", err)
		}
		txf := &future.Tx{Tx: tx, TxKey: txKey}
		txf.Init()
		s.txFuture <- txf
		if err := txf.Error(); err != nil {
			return resp, status.Errorf(codes.Internal, "add tx failed: %v", err)
		}
	case rpcpb.NotifyMsgType_STATEMENT:
		stmt, err := ultpb.DecodeStatement(req.Data)
		if err != nil {
			return resp, status.Error(codes.InvalidArgument, "decode statement failed")
		}
		sf := &future.Statement{Stmt: stmt}
		sf.Init()
		s.stmtFuture <- sf
		if err := sf.Error(); err != nil {
			return resp, status.Errorf(codes.Internal, "add statement failed: %v", err)
		}
	}

	return resp, nil
}
