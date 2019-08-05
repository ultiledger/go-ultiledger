package rpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Hello checks the health of remote peer and at the
// same time exchanges nodeID (public key) between peers.
func Hello(client rpcpb.NodeClient, md metadata.MD, networkID string) (string, string, error) {
	if networkID == "" {
		return "", "", ErrEmptyNetworkID
	}

	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Second))
	defer cancel()

	var header metadata.MD

	req := rpcpb.HelloRequest{NetworkID: networkID}
	_, err := client.Hello(ctx, &req, grpc.Header(&header))
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			log.Errorf("say hello to peer failed: %v", st.Message())
		}
		return "", "", err
	}
	if len(header.Get("addr")) == 0 || len(header.Get("nodeid")) == 0 {
		return "", "", errors.New("empty peerip or nodeid")
	}

	return header.Get("addr")[0], header.Get("nodeid")[0], nil
}
