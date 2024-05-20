package selector

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"
)

// ErrNoAvailable is no available node.
var ErrNoAvailable = errors.ServiceUnavailable("no_available_node", "")

// Selector is node pick balancer.
type Selector interface {
	Rebalancer

	// Select 选择一个节点
	// Select nodes
	// if err == nil, selected and done must not be empty.
	Select(ctx context.Context, opts ...SelectOption) (selected Node, done DoneFunc, err error)
}

// Rebalancer 节点负载均衡器，更新内部服务节点
// Rebalancer is nodes rebalancer.
type Rebalancer interface {
	// Apply is apply all nodes when any changes happen
	Apply(nodes []Node)
}

// Builder selector构建接口
// Builder build selector
type Builder interface {
	Build() Selector
}

// Node 定义节点接口，代表一个服务节点
// 注意Node是在负载均衡中代表一个节点的信息，ServiceInstance 是在服务发现中代表节点中的信息。 由于服务发现和负载均衡，经常一起使用，所以这两个示例经常弄混。
// 一般是通过服务发现获取到 ServiceInstance， 然后转换成Node， 将Node存储到本地列表中，然后做负载均衡。
// selector.NewNode() 方法就是将 ServiceInstance 转换为Node的方法。

// Node is node interface.
type Node interface {
	// Scheme is service node scheme
	Scheme() string

	// Address is the unique address under the same service
	Address() string

	// ServiceName is service name
	ServiceName() string

	// InitialWeight is the initial value of scheduling weight
	// if not set return nil
	InitialWeight() *int64

	// Version is service node version
	Version() string

	// Metadata is the kv pair metadata associated with the service instance.
	// version,namespace,region,protocol etc..
	Metadata() map[string]string
}

// DoneInfo is callback info when RPC invoke done.
type DoneInfo struct {
	// Response Error
	Err error
	// Response Metadata
	ReplyMD ReplyMD

	// BytesSent indicates if any bytes have been sent to the server.
	BytesSent bool
	// BytesReceived indicates if any byte has been received from the server.
	BytesReceived bool
}

// ReplyMD is Reply Metadata.
type ReplyMD interface {
	Get(key string) string
}

// DoneFunc is callback function when RPC invoke done.
type DoneFunc func(ctx context.Context, di DoneInfo)
