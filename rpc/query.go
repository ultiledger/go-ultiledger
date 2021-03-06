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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	pb "github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

type (
	Quorum = ultpb.Quorum
	TxSet  = ultpb.TxSet
	Ledger = ultpb.Ledger
)

// Query quorum information from peers.
func QueryQuorum(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string, networkID string) (*Quorum, error) {
	if networkID == "" {
		return nil, ErrEmptyNetworkID
	}
	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}
	if signature == "" {
		return nil, ErrEmptySignature
	}

	req := &rpcpb.QueryRequest{
		NetworkID: networkID,
		MsgType:   rpcpb.QueryMsgType_QUORUM,
		Data:      payload,
		Signature: signature,
	}

	msg, err := query(clients, md, req)
	if err != nil {
		return nil, fmt.Errorf("query info failed: %v", err)
	}

	quorum := msg.(*Quorum)

	return quorum, nil
}

// Query txset information from peers.
func QueryTxSet(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string, networkID string) (*TxSet, error) {
	if networkID == "" {
		return nil, ErrEmptyNetworkID
	}
	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}
	if signature == "" {
		return nil, ErrEmptySignature
	}

	req := &rpcpb.QueryRequest{
		NetworkID: networkID,
		MsgType:   rpcpb.QueryMsgType_TXSET,
		Data:      payload,
		Signature: signature,
	}

	msg, err := query(clients, md, req)
	if err != nil {
		return nil, fmt.Errorf("query txset failed: %v", err)
	}

	txset := msg.(*TxSet)

	return txset, nil
}

// Query ledger information from peers.
func QueryLedger(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string, networkID string) (*Ledger, error) {
	if networkID == "" {
		return nil, ErrEmptyNetworkID
	}
	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}
	if signature == "" {
		return nil, ErrEmptySignature
	}

	req := &rpcpb.QueryRequest{
		NetworkID: networkID,
		MsgType:   rpcpb.QueryMsgType_LEDGER,
		Data:      payload,
		Signature: signature,
	}

	msg, err := query(clients, md, req)
	if err != nil {
		return nil, fmt.Errorf("query txset failed: %v", err)
	}

	ledger := msg.(*Ledger)

	return ledger, nil
}

// Query needed information from peers iteratively
// and return immediately after receiving the info.
func query(clients []rpcpb.NodeClient, md metadata.MD, req *rpcpb.QueryRequest) (pb.Message, error) {
	if len(clients) == 0 {
		return nil, errors.New("no live clients")
	}

	var message pb.Message
	var err error
	var b []byte

	// Randomly shuffle clients to amortize queries.
	rand.Seed(time.Now().Unix())
	indices := rand.Perm(len(clients))

	for _, i := range indices {
		c := clients[i]
		b, err = queryPeer(c, md, req)
		if err != nil {
			st := status.Convert(err)
			if st.Code() != codes.NotFound {
				log.Errorf("query peer failed: %v", err)
			}
			continue
		}
		if len(b) == 0 {
			continue
		}
		switch req.MsgType {
		case rpcpb.QueryMsgType_QUORUM:
			message, err = ultpb.DecodeQuorum(b)
		case rpcpb.QueryMsgType_TXSET:
			message, err = ultpb.DecodeTxSet(b)
		case rpcpb.QueryMsgType_LEDGER:
			message, err = ultpb.DecodeLedger(b)
		default:
			return nil, errors.New("unknown query msg type")
		}
		if err != nil {
			log.Errorf("decode msg failed: %v", err)
			continue
		}
		return message, nil
	}

	return nil, errors.New("resource not found")
}

func queryPeer(client rpcpb.NodeClient, md metadata.MD, req *rpcpb.QueryRequest) ([]byte, error) {
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(time.Second))
	defer cancel()

	resp, err := client.Query(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}
