package main

import (
	"net/http"
	"net/http/pprof"

	"github.com/coinbase/rosetta-sdk-go/server"
)

type pprofController struct {
	routes []server.Route
}

// See:
// - https://stackoverflow.com/a/71032595/1475331
// - https://pkg.go.dev/net/http/pprof
func newPprofController() *pprofController {
	routes := []server.Route{
		{
			Method:      "GET",
			Pattern:     "/debug/pprof/",
			HandlerFunc: http.HandlerFunc(pprof.Index),
		},
		{
			Method:      "GET",
			Pattern:     "/debug/pprof/cmdline",
			HandlerFunc: http.HandlerFunc(pprof.Cmdline),
		},
		{
			Method:      "GET",
			Pattern:     "/debug/pprof/profile",
			HandlerFunc: http.HandlerFunc(pprof.Profile),
		},
		{
			Method:      "GET",
			Pattern:     "/debug/pprof/symbol",
			HandlerFunc: http.HandlerFunc(pprof.Symbol),
		},
		{
			Method:      "GET",
			Pattern:     "/debug/pprof/trace",
			HandlerFunc: http.HandlerFunc(pprof.Trace),
		},
		{
			Method:      "GET",
			Pattern:     "/debug/pprof/{cmd}",
			HandlerFunc: http.HandlerFunc(pprof.Index),
		},
	}

	return &pprofController{
		routes: routes,
	}
}

func (r *pprofController) Routes() server.Routes {
	return r.routes
}
