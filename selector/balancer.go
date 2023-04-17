package selector

import (
	"context"
	"time"
)

// Balancer is balancer interface
type Balancer interface {
	// Pick 从负载节点中，选择出一个负载节点
	Pick(ctx context.Context, nodes []WeightedNode) (selected WeightedNode, done DoneFunc, err error)
}

// BalancerBuilder build balancer
type BalancerBuilder interface {
	Build() Balancer
}

// WeightedNode 实时计算调度权重的节点

// WeightedNode calculates scheduling weight in real time
type WeightedNode interface {
	Node

	// Raw 返回原始的Node
	// Raw returns the original node
	Raw() Node

	// Weight 该方法可以实时计算节点的负载(比如在p2c中，在两个节点中，选择负载低的)
	// Weight is the runtime calculated weight
	Weight() float64

	// Pick 选择节点: 这个方法，并不是做负载均衡操作。名字和注释容易迷惑人，它的作用是在负载均衡选择了当前节点，请求开始前记录一些东西，在请求开始后，有记录一些东西。 记录的东西主要用于负载计算
	// Pick the node
	Pick() DoneFunc

	// PickElapsed 自最近一次(或者说上一次）选择以来经过的时间
	// PickElapsed is time elapsed since the latest pick
	PickElapsed() time.Duration
}

// WeightedNodeBuilder 通过Node 构建一个WeightNode
// WeightedNodeBuilder is WeightedNode Builder
type WeightedNodeBuilder interface {
	Build(Node) WeightedNode
}
