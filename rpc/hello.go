package rpc

import (
	"context"
	"errors"
	"time"

	pb "github.com/ultiledger/go-ultiledger/rpc/rpcpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Hello checks the health of remote peer and at the
// same time exchanges nodeID (public key) between peers
func Hello(client pb.NodeClient, md metadata.MD) (string, string, error) {
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Second))
	defer cancel()

	var header metadata.MD

	req := pb.HelloRequest{}
	_, err := client.Hello(ctx, &req, grpc.Header(&header))
	if err != nil {
		return "", "", err
	}
	if len(header.Get("Addr")) == 0 || len(header.Get("NodeID")) == 0 {
		return "", "", errors.New("empty peer IP or NodeID")
	}

	return header.Get("Addr")[0], header.Get("NodeID")[0], nil
}
