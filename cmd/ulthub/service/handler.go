package service

import (
	"net/http"

	"github.com/emicklei/go-restful"
)

// NewHandler creates a customized http handler to the http server.
func NewHandler(coreEndpoints string) http.Handler {
	hub := NewHub(coreEndpoints)

	ws := new(restful.WebService)
	ws.Path("/ultiledger").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("/tx").To(hub.SummitTx))
	ws.Route(ws.GET("/tx").To(hub.QueryTx))

	container := restful.NewContainer()
	container.Add(ws)

	return container
}
