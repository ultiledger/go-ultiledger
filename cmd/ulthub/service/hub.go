package app

import (
	"github.com/emicklei/go-restful"
)

// Hub manages the gRPC connections to core servers and
// works as a load balancer to the backend core servers.
type Hub struct {
	coreEndpoints string
}

func NewHub(coreEndpoints string) *Hub {
	return &Hub{coreEndpoints: coreEndpoints}
}

// SummitTx summits the tx to core servers and return approriate
// response messages to client.
func (h *Hub) SummitTx(request *restful.Request, response *restful.Response) {}

// QueryTx query the tx status from core servers and return current
// tx status.
func (h *Hub) QueryTx(request *restful.Request, response *restful.Response) {}
