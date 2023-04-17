package http

import "net/http"

// FilterFunc is a function which receives an http.Handler and returns another http.Handler.
type FilterFunc func(http.Handler) http.Handler

// 将链式的中间件（数组），组成洋葱状。 数组index越大，则处于洋葱的越内测。
// 通俗的讲，就是讲多个中间件，形成一个洋葱状的中间件。

// FilterChain returns a FilterFunc that specifies the chained handler for HTTP Router.
func FilterChain(filters ...FilterFunc) FilterFunc { // 这里包一层，目的是为了传入参数filters
	return func(next http.Handler) http.Handler {
		for i := len(filters) - 1; i >= 0; i-- {
			next = filters[i](next)
		}
		return next
	}
}
