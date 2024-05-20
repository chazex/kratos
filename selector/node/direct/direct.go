package direct

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/selector"
)

const (
	defaultWeight = 100
)

var (
	_ selector.WeightedNode        = (*Node)(nil)
	_ selector.WeightedNodeBuilder = (*Builder)(nil)
)

// Node is endpoint instance
type Node struct {
	selector.Node

	// last lastPick timestamp
	lastPick int64
}

// Builder is direct node builder
type Builder struct{}

// Build create node
func (*Builder) Build(n selector.Node) selector.WeightedNode {
	return &Node{Node: n, lastPick: 0}
}

func (n *Node) Pick() selector.DoneFunc {
	now := time.Now().UnixNano()
	atomic.StoreInt64(&n.lastPick, now)
	return func(ctx context.Context, di selector.DoneInfo) {}
}

// 计算节点的负载，因为是direct，所以使用默认值（defaultWeight）

// Weight is node effective weight
func (n *Node) Weight() float64 {
	if n.InitialWeight() != nil {
		return float64(*n.InitialWeight())
	}
	return defaultWeight
}

// PickElapsed 这个节点在最近两次被pick的时间间隔
func (n *Node) PickElapsed() time.Duration {
	return time.Duration(time.Now().UnixNano() - atomic.LoadInt64(&n.lastPick))
}

func (n *Node) Raw() selector.Node {
	return n.Node
}
