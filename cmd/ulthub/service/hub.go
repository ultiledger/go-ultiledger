package service

import (
	"github.com/emicklei/go-restful"

	"github.com/ultiledger/go-ultiledger/log"
)

// Hub manages the gRPC connections to ult servers and
// works as a load balancer to the backend ult servers.
type Hub struct {
	coreEndpoints string
}

func NewHub(coreEndpoints string) *Hub {
	return &Hub{coreEndpoints: coreEndpoints}
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
