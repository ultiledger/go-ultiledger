package client

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

type TxStatusCode uint8

const (
	NotExist TxStatusCode = iota
	Rejected
	Accepted
	Confirmed
	Failed
	Unknown
)

// TxStatus represents the status of current tx in node.
type TxStatus struct {
	StatusCode   TxStatusCode
	ErrorMessage string
}

// Hub manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type GrpcClient struct {
	networkID     string
	accountID     string
	coreEndpoints string
	client        rpcpb.NodeClient
}

func New(networkID, accountID, coreEndpoints string) (*GrpcClient, error) {
	// connect to core servers
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
// response messages to client.
func (c *GrpcClient) SubmitTx(txKey string, signature string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	req := &rpcpb.SubmitTxRequest{
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

// QueryTx query the tx status from ult servers and return current
// tx status.
func (c *GrpcClient) QueryTx(txKey string) (*TxStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second))
	defer cancel()

	req := &rpcpb.QueryTxRequest{TxKey: txKey}
	resp, err := c.client.QueryTx(ctx, req)
	if err != nil {
		return nil, err
	}

	status := &TxStatus{
		ErrorMessage: resp.TxStatus.ErrorMessage,
	}
	switch resp.TxStatus.StatusCode {
	case rpcpb.TxStatusCode_NOTEXIST:
		status.StatusCode = NotExist
	case rpcpb.TxStatusCode_REJECTED:
		status.StatusCode = Rejected
	case rpcpb.TxStatusCode_ACCEPTED:
		status.StatusCode = Accepted
	case rpcpb.TxStatusCode_CONFIRMED:
		status.StatusCode = Confirmed
	case rpcpb.TxStatusCode_FAILED:
		status.StatusCode = Failed
	default:
		status.StatusCode = Unknown
	}

	return status, nil
}
