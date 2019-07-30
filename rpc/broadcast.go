package rpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

var (
	ErrEmptyNetworkID = errors.New("empty network id")
	ErrEmptyPayload   = errors.New("empty payload")
	ErrEmptySignature = errors.New("empty digital signature")
)

// For reusing broadcast signal type.
var taskPool *sync.Pool

func init() {
	taskPool = &sync.Pool{
		New: func() interface{} {
			return new(task)
		},
	}
}

// Broadcast consensus statements.
func BroadcastStatement(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string, networkID string) error {
	if networkID == "" {
		return ErrEmptyNetworkID
	}
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	if signature == "" {
		return ErrEmptySignature
	}
	req := &rpcpb.NotifyRequest{
		NetworkID: networkID,
		MsgType:   rpcpb.NotifyMsgType_STATEMENT,
		Data:      payload,
		Signature: signature,
	}
	err := broadcast(clients, md, req)
	if err != nil {
		return fmt.Errorf("broadcast failed: %v", err)
	}
	return nil
}

// Broadcast transaction.
func BroadcastTx(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string, networkID string) error {
	if networkID == "" {
		return ErrEmptyNetworkID
	}
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	if signature == "" {
		return ErrEmptySignature
	}
	req := &rpcpb.NotifyRequest{
		NetworkID: networkID,
		MsgType:   rpcpb.NotifyMsgType_TX,
		Data:      payload,
		Signature: signature,
	}
	err := broadcast(clients, md, req)
	if err != nil {
		return fmt.Errorf("broadcast failed: %v", err)
	}
	return nil
}

// Broadcast supplied request concurrently.
func broadcast(clients []rpcpb.NodeClient, md metadata.MD, req *rpcpb.NotifyRequest) error {
	done := make(chan bool)
	tasks := prepareTask(done, clients, md, req)
	workers := make([]<-chan *rpcpb.NotifyResponse, 10) // hard code for now
	for i := 0; i < 10; i++ {
		workers[i] = runTask(done, tasks)
	}
	for _ = range mergeResponse(done, workers...) {
	}
	close(done)
	return nil
}

// Internal concurrent broadcast task.
type task struct {
	client   rpcpb.NodeClient
	metadata metadata.MD
	req      *rpcpb.NotifyRequest
}

// Prepare broadcast task for concurrent processing
func prepareTask(done <-chan bool, clients []rpcpb.NodeClient, md metadata.MD, req *rpcpb.NotifyRequest) <-chan *task {
	taskChan := make(chan *task)
	go func() {
		for _, c := range clients {
			t := taskPool.Get().(*task)
			t.client = c
			t.metadata = md
			t.req = req
			select {
			case <-done:
				return
			case taskChan <- t:
			}
		}
	}()
	return taskChan
}

// Run task by invoking notify method.
func runTask(done <-chan bool, taskChan <-chan *task) <-chan *rpcpb.NotifyResponse {
	responseChan := make(chan *rpcpb.NotifyResponse)
	notify := func(t *task) *rpcpb.NotifyResponse {
		ctx := metadata.NewOutgoingContext(context.Background(), t.metadata)
		ctx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Second))
		defer cancel()
		// we still have to return the response even if
		// error happend and in this case response is nil
		resp, err := t.client.Notify(ctx, t.req)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				log.Errorf("notify peer failed: %v", st.Message())
			}
		}
		return resp
	}
	go func() {
		for t := range taskChan {
			select {
			case <-done:
				return
			case responseChan <- notify(t):
				// return task to pool after using
				taskPool.Put(t)
			}
		}
	}()
	return responseChan
}

// Merge responses from multiple workers to return a merged response channel
func mergeResponse(done <-chan bool, responseChans ...<-chan *rpcpb.NotifyResponse) <-chan *rpcpb.NotifyResponse {
	var wg sync.WaitGroup
	wg.Add(len(responseChans))
	result := make(chan *rpcpb.NotifyResponse)
	multiplex := func(responseChan <-chan *rpcpb.NotifyResponse) {
		defer wg.Done()
		for resp := range responseChan {
			select {
			case <-done:
				return
			case result <- resp:
			}
		}
	}
	for _, c := range responseChans {
		go multiplex(c)
	}
	go func() {
		wg.Wait()
		close(result)
	}()
	return result
}
