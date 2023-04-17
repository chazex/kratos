package selector

import "context"

// NodeFilter 节点过滤函数
// NodeFilter is select filter.
type NodeFilter func(context.Context, []Node) []Node
