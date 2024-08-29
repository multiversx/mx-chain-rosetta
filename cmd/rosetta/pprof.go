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
			Method:      http.MethodGet,
			Pattern:     "/debug/pprof/",
			HandlerFunc: http.HandlerFunc(pprof.Index),
		},
		{
			Method:      http.MethodGet,
			Pattern:     "/debug/pprof/cmdline",
			HandlerFunc: http.HandlerFunc(pprof.Cmdline),
		},
		{
			Method:      http.MethodGet,
			Pattern:     "/debug/pprof/profile",
			HandlerFunc: http.HandlerFunc(pprof.Profile),
		},
		{
			Method:      http.MethodGet,
			Pattern:     "/debug/pprof/symbol",
			HandlerFunc: http.HandlerFunc(pprof.Symbol),
		},
		{
			Method:      http.MethodGet,
			Pattern:     "/debug/pprof/trace",
			HandlerFunc: http.HandlerFunc(pprof.Trace),
		},
		{
			Method:      http.MethodGet,
			Pattern:     "/debug/pprof/{cmd}",
			HandlerFunc: http.HandlerFunc(pprof.Index),
		},
	}

	return &pprofController{
		routes: routes,
	}
}

// Routes returns the routes for the pprof controller
func (r *pprofController) Routes() server.Routes {
	return r.routes
}
