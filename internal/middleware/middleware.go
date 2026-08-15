// internal/middleware/chain.go
package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

type ChainBuilder struct {
	middlewares []Middleware
}

func New(middlewares ...Middleware) ChainBuilder {
	return ChainBuilder{
		middlewares: append([]Middleware(nil), middlewares...),
	}
}

func (c ChainBuilder) Use(m ...Middleware) ChainBuilder {
	newMiddlewares := make([]Middleware, 0, len(c.middlewares)+len(m))
	newMiddlewares = append(newMiddlewares, c.middlewares...)
	newMiddlewares = append(newMiddlewares, m...)

	return ChainBuilder{
		middlewares: newMiddlewares,
	}
}

func (c ChainBuilder) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}
	return h
}
