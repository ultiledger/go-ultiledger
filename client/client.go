package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/client/types"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"github.com/ultiledger/go-ultiledger/ultpb"
)

// TxStatus represents the status of current tx in node.
type TxStatus struct {
	StatusCode   types.TxStatusCode
	ErrorMessage string
}

// GrpcClient manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type GrpcClient struct {
	networkID     string
	accountID     string
	coreEndpoints string
	client        rpcpb.NodeClient
}

// New creates a GrpcClient to the given target servers.
func New(networkID, accountID, coreEndpoints string) (*GrpcClient, error) {
	// Connect to node servers.
	r := NewResolver()
	b := grpc.RoundRobin(r)
	conn, err := grpc.Dial(coreEndpoints, grpc.WithInsecure(), grpc.WithBalancer(b), grpc.WithBlock(), grpc.WithTimeout(time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to core servers failed: %v", err)
	}
	client := rpcpb.NewNodeClient(conn)
	gc := &GrpcClient{
		networkID:     networkID,
		accountID:     accountID,
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
func (c *GrpcClient) QueryTx(txKey string) (*TxStatus, error) {
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

	status := &TxStatus{
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

	return status, nil
}

// GetAccount gets the account with the requested account id.
func (c *GrpcClient) GetAccount(accountID string) (*types.Account, error) {
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

	account := &types.Account{
		AccountID:        acc.AccountID,
		Balance:          acc.Balance,
		Signer:           acc.Signer,
		SeqNum:           acc.SeqNum,
		EntryCount:       acc.EntryCount,
		BuyingLiability:  acc.Liability.Buying,
		SellingLiability: acc.Liability.Selling,
	}

	return account, nil
}
