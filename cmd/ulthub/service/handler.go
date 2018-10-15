package service

import (
	"net/http"
	"time"

	"github.com/emicklei/go-restful"

	"github.com/ultiledger/go-ultiledger/log"
)

// NewHandler creates a customized http handler to the http server.
func NewHandler(coreEndpoints string) http.Handler {
	hub := NewHub(coreEndpoints)

	ws := new(restful.WebService)
	ws.Path("/ultiledger").
		Consumes(restful.MIME_JSON).
		Produces(restful.MIME_JSON)
	ws.Route(ws.POST("/tx").To(hub.SummitTx))
	ws.Route(ws.GET("/tx").To(hub.QueryTx)).Filter(responseFilter)

	container := restful.NewContainer()
	container.Add(ws)

	return container
}

func responseFilter(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	start := time.Now()
	chain.ProcessFilter(req, resp)
	end := time.Now().Sub(start)

	if resp.StatusCode() != 200 {
		log.Errorw("failed", "statusCode", resp.StatusCode(), "method", req.Request.Method, "URL", req.Request.URL, "errorMsg", resp.Error(), "responseTime", end)
	}
	log.Infow("success", "statusCode", resp.StatusCode(), "method", req.Request.Method, "URL", req.Request.URL, "responseTime", end)
}
