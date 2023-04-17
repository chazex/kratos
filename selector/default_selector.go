package selector

import (
	"context"
	"sync/atomic"
)

// Default 是selector的默认实现。 它内部通过Balancer进行负载均衡
// selector除了 balancer，还有过滤作用

// Default is composite selector.
type Default struct {
	// 由于Balancer接口的Pick方法，需要输入[]WeightedNode，并返回一个WeightedNode，所以这里要存储WeightedNode的构建方法
	NodeBuilder WeightedNodeBuilder
	// 通过这个balancer来做负载均衡（调用其Pick()方法）
	Balancer Balancer

	// 通过Apply方法，将WeightedNode存储到nodes中
	nodes atomic.Value
}

// Select is select one node.
func (d *Default) Select(ctx context.Context, opts ...SelectOption) (selected Node, done DoneFunc, err error) {
	var (
		options    SelectOptions
		candidates []WeightedNode
	)
	// 加载所有节点
	nodes, ok := d.nodes.Load().([]WeightedNode)
	if !ok {
		return nil, nil, ErrNoAvailable
	}
	for _, o := range opts {
		o(&options)
	}
	// 1. 走过滤器
	if len(options.NodeFilters) > 0 {
		newNodes := make([]Node, len(nodes))
		for i, wc := range nodes {
			newNodes[i] = wc
		}
		// 过滤器
		for _, filter := range options.NodeFilters {
			newNodes = filter(ctx, newNodes)
		}
		// 得到候选者
		candidates = make([]WeightedNode, len(newNodes))
		for i, n := range newNodes {
			candidates[i] = n.(WeightedNode)
		}
	} else {
		candidates = nodes
	}

	if len(candidates) == 0 {
		// 没有候选者
		return nil, nil, ErrNoAvailable
	}
	// 调用负载均衡器，执行对应的负载均衡策略，从候选节点中，选择一个节点
	wn, done, err := d.Balancer.Pick(ctx, candidates) // 由负载均衡器，从候选节点中pick一个出来
	if err != nil {
		return nil, nil, err
	}
	p, ok := FromPeerContext(ctx)
	if ok {
		p.Node = wn.Raw()
	}
	return wn.Raw(), done, nil
}

// Apply update nodes info.
func (d *Default) Apply(nodes []Node) {
	// 就是用新的node列表，替换老的node列表
	weightedNodes := make([]WeightedNode, 0, len(nodes))
	for _, n := range nodes {
		weightedNodes = append(weightedNodes, d.NodeBuilder.Build(n))
	}
	// TODO: Do not delete unchanged nodes
	d.nodes.Store(weightedNodes)
}

// DefaultBuilder is de
type DefaultBuilder struct {
	Node     WeightedNodeBuilder
	Balancer BalancerBuilder
}

// Build create builder
func (db *DefaultBuilder) Build() Selector {
	return &Default{
		NodeBuilder: db.Node,
		Balancer:    db.Balancer.Build(),
	}
}
