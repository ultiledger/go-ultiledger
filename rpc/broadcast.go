package rpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/metadata"

	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

var (
	ErrUnknownMsgType = errors.New("unknown broadcast message type")
	ErrEmptyPayload   = errors.New("empty payload")
	ErrEmptySignature = errors.New("empty digital signature")
)

// for reusing broadcast signal type
var taskPool *sync.Pool

func init() {
	taskPool = &sync.Pool{
		New: func() interface{} {
			return new(task)
		},
	}
}

// broadcast nomination statements
func BroadcastNomination(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	if signature == "" {
		return ErrEmptySignature
	}
	req := &rpcpb.NotifyRequest{
		MsgType:   rpcpb.NotifyMsgType_NOMINATE,
		Data:      payload,
		Signature: signature,
	}
	err := broadcast(clients, md, req)
	if err != nil {
		return err
	}
	return nil
	return nil
}

// broadcast transaction
func BroadcastTx(clients []rpcpb.NodeClient, md metadata.MD, payload []byte, signature string) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	if signature == "" {
		return ErrEmptySignature
	}
	req := &rpcpb.NotifyRequest{
		MsgType:   rpcpb.NotifyMsgType_Tx,
		Data:      payload,
		Signature: signature,
	}
	err := broadcast(clients, md, req)
	if err != nil {
		return err
	}
	return nil
}

// broadcast supplied request concurrently
func broadcast(clients []rpcpb.NodeClient, md metadata.MD, req *rpcpb.NotifyRequest) error {
	done := make(chan bool)
	tasks := prepareTask(done, clients, md, req)
	workers := make([]<-chan *rpcpb.NotifyResponse, 10) // hard code for now
	for i := 0; i < 10; i++ {
		workers[i] = runTask(done, tasks)
	}
	for resp := range mergeResponse(done, workers...) {
		fmt.Println(resp) // for eliminating compile error
	}
	close(done)
	return nil
}

// internal concurrent broadcast task
type task struct {
	client   rpcpb.NodeClient
	metadata metadata.MD
	req      *rpcpb.NotifyRequest
}

// prepare broadcast task for concurrent processing
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

// run task by invoking notify method
func runTask(done <-chan bool, taskChan <-chan *task) <-chan *rpcpb.NotifyResponse {
	responseChan := make(chan *rpcpb.NotifyResponse)
	notify := func(t *task) *rpcpb.NotifyResponse {
		ctx := metadata.NewOutgoingContext(context.Background(), t.metadata)
		ctx, cancel := context.WithTimeout(ctx, time.Duration(1*time.Second))
		defer cancel()
		// we still have to return the response even if
		// error happend and in this case response is nil
		resp, _ := t.client.Notify(ctx, t.req)
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

// merge responses from multiple workers to return a merged response channel
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
