package grpc

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/metadata"

	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
	"github.com/go-kratos/kratos/v2/transport"
)

const (
	balancerName = "selector"
)

var (
	_ base.PickerBuilder = (*balancerBuilder)(nil)
	_ balancer.Picker    = (*balancerPicker)(nil)
)

func init() {
	// 借助grpc原生的baseBalancer做封装
	b := base.NewBalancerBuilder(
		balancerName,
		&balancerBuilder{
			builder: selector.GlobalSelector(),
		},
		base.Config{HealthCheck: true},
	)
	balancer.Register(b)
}

// 在这里称为balancerBuilder，实际在grpc中，是baseBalancer中的pickerBuilder
type balancerBuilder struct {
	builder selector.Builder
}

// 在什么情况下，这个方法会被调用？ 应该是grpc中服务节点触发变化的时候

// Build creates a grpc Picker.
func (b *balancerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		// Block the RPC until a new picker is available via UpdateState().
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	// 根据就绪的grpc.SubConn构建kratos的节点列表。
	nodes := make([]selector.Node, 0, len(info.ReadySCs))
	for conn, info := range info.ReadySCs {
		ins, _ := info.Address.Attributes.Value("rawServiceInstance").(*registry.ServiceInstance)
		nodes = append(nodes, &grpcNode{
			Node:    selector.NewNode("grpc", info.Address.Addr, ins),
			subConn: conn,
		})
	}
	p := &balancerPicker{
		selector: b.builder.Build(),
	}
	p.selector.Apply(nodes)
	return p
}

// balancerPicker is a grpc picker.
type balancerPicker struct {
	selector selector.Selector
}

// Pick pick instances.
func (p *balancerPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	var filters []selector.NodeFilter
	if tr, ok := transport.FromClientContext(info.Ctx); ok {
		if gtr, ok := tr.(*Transport); ok {
			filters = gtr.NodeFilters()
		}
	}

	// done 执行完成grpc请求之后，调用done方法，来做一些统计，用于计算负载吧？
	n, done, err := p.selector.Select(info.Ctx, selector.WithNodeFilter(filters...))
	if err != nil {
		return balancer.PickResult{}, err
	}

	return balancer.PickResult{
		SubConn: n.(*grpcNode).subConn,
		Done: func(di balancer.DoneInfo) {
			done(info.Ctx, selector.DoneInfo{
				Err:           di.Err,
				BytesSent:     di.BytesSent,
				BytesReceived: di.BytesReceived,
				ReplyMD:       Trailer(di.Trailer),
			})
		},
	}, nil
}

// Trailer is a grpc trailer MD.
type Trailer metadata.MD

// Get get a grpc trailer value.
func (t Trailer) Get(k string) string {
	v := metadata.MD(t).Get(k)
	if len(v) > 0 {
		return v[0]
	}
	return ""
}

// selector.Node做了一个扩展，用于对接grpc。 因为grpc中picker得到的balancer.PickResult，需要有grpc.SubConn

type grpcNode struct {
	selector.Node
	subConn balancer.SubConn
}
