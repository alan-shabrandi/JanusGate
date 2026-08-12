package router

import (
	"errors"
	"net/http"
	"strings"

	"janusgate/internal/config"
)

var (
	ErrRouteNotFound = errors.New("route not found")
	ErrInvalidPath   = errors.New("invalid route path")
)

type Router interface {
	AddRoute(route config.RouteConfig, handler http.Handler) error

	http.Handler
}

type muxRouter struct {
	mux *http.ServeMux
}

func NewRouter() Router {
	return &muxRouter{
		mux: http.NewServeMux(),
	}
}

func (r *muxRouter) AddRoute(route config.RouteConfig, handler http.Handler) error {
	if route.Path == "" {
		return ErrInvalidPath
	}

	pattern := route.Path
	if !strings.HasSuffix(pattern, "/") {
		pattern += "/"
	}

	r.mux.Handle(pattern, handler)
	return nil
}

func (r *muxRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
