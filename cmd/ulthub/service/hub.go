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

package service

import (
	"fmt"
	"time"

	"github.com/emicklei/go-restful"
	"google.golang.org/grpc"

	"github.com/ultiledger/go-ultiledger/log"
	"github.com/ultiledger/go-ultiledger/rpc/rpcpb"
)

// Hub manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type Hub struct {
	coreEndpoints string
	client        rpcpb.NodeClient
}

func NewHub(coreEndpoints string) (*Hub, error) {
	// connect to core servers
	r := NewResolver()
	b := grpc.RoundRobin(r)
	conn, err := grpc.Dial(coreEndpoints, grpc.WithInsecure(), grpc.WithBalancer(b), grpc.WithBlock(), grpc.WithTimeout(time.Second))
	if err != nil {
		return nil, fmt.Errorf("connect to core servers failed: %v", err)
	}
	client := rpcpb.NewNodeClient(conn)
	return &Hub{coreEndpoints: coreEndpoints, client: client}, nil
}

// SummitTx summits the tx to ult servers and return appropriate
// response messages to client.
func (h *Hub) SummitTx(request *restful.Request, response *restful.Response) {
	log.Info("got summit tx request")
}

// QueryTx query the tx status from ult servers and return current
// tx status.
func (h *Hub) QueryTx(request *restful.Request, response *restful.Response) {
	log.Info("got query tx request")
}
