package http

import (
	"net/http"
	"path"
	"sync"
)

// WalkRouteFunc is the type of the function called for each route visited by Walk.
type WalkRouteFunc func(RouteInfo) error

// RouteInfo is an HTTP route info.
type RouteInfo struct {
	Path   string
	Method string
}

// HandlerFunc defines a function to serve HTTP requests.
type HandlerFunc func(Context) error

// Router is an HTTP router.
type Router struct {
	prefix  string
	pool    sync.Pool
	srv     *Server
	filters []FilterFunc
}

func newRouter(prefix string, srv *Server, filters ...FilterFunc) *Router {
	r := &Router{
		prefix:  prefix,
		srv:     srv,
		filters: filters,
	}
	r.pool.New = func() interface{} {
		return &wrapper{router: r}
	}
	return r
}

// Group returns a new router group.
func (r *Router) Group(prefix string, filters ...FilterFunc) *Router {
	var newFilters []FilterFunc
	newFilters = append(newFilters, r.filters...)
	newFilters = append(newFilters, filters...)
	return newRouter(path.Join(r.prefix, prefix), r.srv, newFilters...)
}

// Handle registers a new route with a matcher for the URL path and method.
func (r *Router) Handle(method, relativePath string, h HandlerFunc, filters ...FilterFunc) {
	// 参数h是用户处理函数(实际上是业务中间件+处理逻辑)，即proto文件定义的接口的具体实现，再用业务层的中间件进行了一层层的包裹
	// 由于上层传过来的是kratos的HandlerFunc类型，所以要转换成net.http.Hander类型。因为这个函数要注册到gorilla/mux里面，所以他要遵循规则（路由处理函数要实现net.http.Hander）
	next := http.Handler(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := r.pool.Get().(Context)
		// 由于这块代码的上游是路由中间件（gorilla/mux的middleware），所以这里的res和req是来自于kratos.server.filter()方法中，重置的req和res
		ctx.Reset(res, req) //由于ctx是从pool中取到的，所以用之前，先把res,req重置一下
		// 调用业务
		if err := h(ctx); err != nil {
			// 业务处理函数返回错误，使用server的error encoder
			r.srv.ene(res, req, err)
		}
		ctx.Reset(nil, nil) // 同理，用完之后，扔回pool之前，要reset为nil
		r.pool.Put(ctx)
	}))
	// 这里的filters，是上游传过来的，但是我找了下代码都是空。而且上游是proto生成的，貌似没啥用。
	// 如果不用proto的方式，自己手动实现http代码，然后注册到server里面，应该会有这种场景，比如stream 式的grpc。
	// 并且这样，就可以自己决定是否传入filters了.
	next = FilterChain(filters...)(next)
	// 这个filters，我也没找到哪里会注册。 估计也是用户自己实现http，然后在Group里面加
	next = FilterChain(r.filters...)(next)
	// 在mux上注册一个新的路由(因为kratos使用的是gorilla/mux，所以最终要是要注册到这上面的)
	r.srv.router.Handle(path.Join(r.prefix, relativePath), next).Methods(method)
}

// GET registers a new GET route for a path with matching handler in the router.
func (r *Router) GET(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodGet, path, h, m...)
}

// HEAD registers a new HEAD route for a path with matching handler in the router.
func (r *Router) HEAD(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodHead, path, h, m...)
}

// POST registers a new POST route for a path with matching handler in the router.
func (r *Router) POST(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodPost, path, h, m...)
}

// PUT registers a new PUT route for a path with matching handler in the router.
func (r *Router) PUT(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodPut, path, h, m...)
}

// PATCH registers a new PATCH route for a path with matching handler in the router.
func (r *Router) PATCH(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodPatch, path, h, m...)
}

// DELETE registers a new DELETE route for a path with matching handler in the router.
func (r *Router) DELETE(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodDelete, path, h, m...)
}

// CONNECT registers a new CONNECT route for a path with matching handler in the router.
func (r *Router) CONNECT(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodConnect, path, h, m...)
}

// OPTIONS registers a new OPTIONS route for a path with matching handler in the router.
func (r *Router) OPTIONS(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodOptions, path, h, m...)
}

// TRACE registers a new TRACE route for a path with matching handler in the router.
func (r *Router) TRACE(path string, h HandlerFunc, m ...FilterFunc) {
	r.Handle(http.MethodTrace, path, h, m...)
}
