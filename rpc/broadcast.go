package rpc

import (
	"errors"

	pb "github.com/golang/protobuf/proto"
	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

var (
	ErrUnknownMsgType = errors.New("unknown broadcast message type")
)

type BroadcastMsgType uint8

const (
	BroadcastNominate BroadcastMsgType = iota
)

func Broadcast(clients []rpcpb.NodeClient, md metadata.MD, msg pb.Message, msgType BroadcastMsgType) error {
	if len(clients) == 0 {
		return errors.New("empty list of clients")
	}
	if msg == nil {
		return errors.New("message is nil")
	}
	var err error
	switch msgType {
	case BroadcastNominate:
		err = broadcastNomination(clients, md, msg)
	default:
		return ErrUnknownMsgType
	}
	return err
}

// broadcast nomination statements
func broadcastNomination(clients []rpcpb.NodeClient, md metadata.MD, msg pb.Message) error {
	return nil
}
