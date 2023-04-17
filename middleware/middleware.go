package middleware

import (
	"context"
)

// Handler defines the handler invoked by Middleware.
type Handler func(ctx context.Context, req interface{}) (interface{}, error)

// Middleware is HTTP/gRPC transport middleware.
type Middleware func(Handler) Handler

// 将链式的中间件（数组），组成洋葱状。 数组index越大，则处于洋葱的越内测。
// 通俗的讲，就是讲多个中间件，形成一个洋葱状的中间件。

// Chain returns a Middleware that specifies the chained handler for endpoint.
func Chain(m ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(m) - 1; i >= 0; i-- {
			next = m[i](next)
		}
		return next
	}
}
