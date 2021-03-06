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

// Package client defines the APIs to communicate with the node.
package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	b58 "github.com/mr-tron/base58/base58"
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/client/types"
	"github.com/ultiledger/go-ultiledger/crypto"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// GrpcClient manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type GrpcClient struct {
	networkID     string
	coreEndpoints string
	client        rpcpb.NodeClient
}

// New creates a GrpcClient to the given target servers.
func New(networkID, coreEndpoints string) (*GrpcClient, error) {
	// Connect to node servers.
	r := NewResolver()
	b := grpc.RoundRobin(r)
	conn, err := grpc.Dial(coreEndpoints, grpc.WithInsecure(), grpc.WithBalancer(b), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to core servers failed: %v", err)
	}

	// Compute the hash of network id.
	netID := crypto.SHA256HashBytes([]byte(networkID))
	netIDStr := b58.Encode(netID[:])

	client := rpcpb.NewNodeClient(conn)
	gc := &GrpcClient{
		networkID:     netIDStr,
		coreEndpoints: coreEndpoints,
		client:        client,
	}
	return gc, nil
}

// SummitTx summits the tx to ult servers and return appropriate
// response messages to the client.
func (c *GrpcClient) SubmitTx(txKey string, signature string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	req := &rpcpb.SubmitTxRequest{
		NetworkID: c.networkID,
		TxKey:     txKey,
		Signature: signature,
		Data:      data,
	}
	_, err := c.client.SubmitTx(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

// QueryTx queries the tx status from ult servers and return current
// tx status.
func (c *GrpcClient) QueryTx(txKey string) (*types.TxStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	req := &rpcpb.QueryTxRequest{
		NetworkID: c.networkID,
		TxKey:     txKey,
	}
	resp, err := c.client.QueryTx(ctx, req)
	if err != nil {
		return nil, err
	}

	status := &types.TxStatus{
		ErrorMessage: resp.TxStatus.ErrorMessage,
	}
	switch resp.TxStatus.StatusCode {
	case rpcpb.TxStatusCode_NOTEXIST:
		status.StatusCode = types.NotExist
	case rpcpb.TxStatusCode_REJECTED:
		status.StatusCode = types.Rejected
	case rpcpb.TxStatusCode_ACCEPTED:
		status.StatusCode = types.Accepted
	case rpcpb.TxStatusCode_CONFIRMED:
		status.StatusCode = types.Confirmed
	case rpcpb.TxStatusCode_FAILED:
		status.StatusCode = types.Failed
	default:
		status.StatusCode = types.Unknown
	}

	if len(resp.TxStatus.Data) > 0 {
		tx, err := ultpb.DecodeTx(resp.TxStatus.Data)
		if err != nil {
			return nil, fmt.Errorf("decode tx failed: %v", err)
		}
		status.Tx = tx
	}

	return status, nil
}

// GetAccount gets the account with the requested account id.
func (c *GrpcClient) GetAccount(accountID string) (*ultpb.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	req := &rpcpb.GetAccountRequest{
		NetworkID: c.networkID,
		AccountID: accountID,
	}
	resp, err := c.client.GetAccount(ctx, req)
	if err != nil {
		return nil, err
	}

	acc, err := ultpb.DecodeAccount(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("decode account failed: %v", err)
	}

	return acc, nil
}

// CreateTestAccount creates a test account for the
// purpose of testing in testnet.
func (c *GrpcClient) CreateTestAccount(accountID string) (*ultpb.Account, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	req := &rpcpb.CreateTestAccountRequest{
		NetworkID: c.networkID,
		AccountID: accountID,
	}
	resp, err := c.client.CreateTestAccount(ctx, req)
	if err != nil {
		return nil, err
	}

	// Wait until the tx be confirmed.
	ticker := time.NewTicker(2 * time.Second)
	timer := time.NewTimer(30 * time.Second)
	for {
		select {
		case <-ticker.C:
			// Check the tx.
			status, err := c.QueryTx(resp.TxKey)
			if err != nil {
				return nil, fmt.Errorf("query tx failed: %v", err)
			}
			switch status.StatusCode {
			case types.NotExist:
				return nil, errors.New("tx not found")
			case types.Rejected:
				return nil, fmt.Errorf("tx rejected: %v", status.ErrorMessage)
			case types.Accepted:
				continue
			case types.Confirmed:
				// Get the account
				account, err := c.GetAccount(accountID)
				if err != nil {
					return nil, fmt.Errorf("get account failed: %v", err)
				}
				return account, nil
			case types.Failed:
				return nil, fmt.Errorf("tx failed: %v", status.ErrorMessage)
			default:
				return nil, errors.New("tx status unknown")
			}
		case <-timer.C:
			return nil, errors.New("query result takes too long")
		}
	}

	return nil, nil
}
