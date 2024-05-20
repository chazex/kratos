package ewma

import (
	"container/list"
	"context"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/selector"
)

const (
	// The mean lifetime of `cost`, it reaches its half-life after Tau*ln(2).
	tau = int64(time.Millisecond * 600)
	// if statistic not collected,we add a big lag penalty to endpoint
	penalty = uint64(time.Second * 10)
)

var (
	_ selector.WeightedNode        = (*Node)(nil)
	_ selector.WeightedNodeBuilder = (*Builder)(nil)
)

// Node 一个后端服务节点实例
// Node is endpoint instance
type Node struct {
	selector.Node

	// client statistic data
	lag     int64
	success uint64
	// 这个节点正在处理的请求数量（只是对于单个client的请求数量），如果有多个客户端的话，数量应该不止这个数
	inflight  int64
	inflights *list.List
	// last collected timestamp
	stamp     int64
	predictTs int64
	predict   int64
	// request number in a period time
	reqs int64
	// last 最近一次被负载均衡器选中的时间戳
	// last lastPick timestamp
	lastPick int64

	errHandler func(err error) (isErr bool)
	lk         sync.RWMutex
}

// Builder is ewma node builder.
type Builder struct {
	ErrHandler func(err error) (isErr bool)
}

// Build create a weighted node.
func (b *Builder) Build(n selector.Node) selector.WeightedNode {
	s := &Node{
		Node:       n,
		lag:        0,
		success:    1000,
		inflight:   1,
		inflights:  list.New(),
		errHandler: b.ErrHandler,
	}
	return s
}

func (n *Node) health() uint64 {
	return atomic.LoadUint64(&n.success)
}

func (n *Node) load() (load uint64) {
	now := time.Now().UnixNano()
	avgLag := atomic.LoadInt64(&n.lag)
	lastPredictTs := atomic.LoadInt64(&n.predictTs)
	predictInterval := avgLag / 5
	if predictInterval < int64(time.Millisecond*5) {
		predictInterval = int64(time.Millisecond * 5)
	}
	if predictInterval > int64(time.Millisecond*200) {
		predictInterval = int64(time.Millisecond * 200)
	}
	if now-lastPredictTs > predictInterval && atomic.CompareAndSwapInt64(&n.predictTs, lastPredictTs, now) {
		var (
			total   int64
			count   int
			predict int64
		)
		n.lk.RLock()
		first := n.inflights.Front()
		for first != nil {
			lag := now - first.Value.(int64)
			if lag > avgLag {
				count++
				total += lag
			}
			first = first.Next()
		}
		if count > (n.inflights.Len()/2 + 1) {
			predict = total / int64(count)
		}
		n.lk.RUnlock()
		atomic.StoreInt64(&n.predict, predict)
	}

	if avgLag == 0 {
		// penalty is the penalty value when there is no data when the node is just started.
		// The default value is 1e9 * 10
		load = penalty * uint64(atomic.LoadInt64(&n.inflight))
		return
	}
	predict := atomic.LoadInt64(&n.predict)
	if predict > avgLag {
		avgLag = predict
	}
	load = uint64(avgLag) * uint64(atomic.LoadInt64(&n.inflight))
	return
}

// Pick pick a node.
func (n *Node) Pick() selector.DoneFunc {
	now := time.Now().UnixNano()
	atomic.StoreInt64(&n.lastPick, now)
	// 正在处理中的请求+1
	atomic.AddInt64(&n.inflight, 1)
	atomic.AddInt64(&n.reqs, 1)
	n.lk.Lock()
	e := n.inflights.PushBack(now)
	n.lk.Unlock()
	return func(ctx context.Context, di selector.DoneInfo) {
		n.lk.Lock()
		// 请求处理完成，清除链表中的元素
		n.inflights.Remove(e)
		n.lk.Unlock()
		// 请求处理完成，处理中的请求-1
		atomic.AddInt64(&n.inflight, -1)

		// 本次请求完成时间
		now := time.Now().UnixNano()
		// get moving average ratio w
		// 保存最新一次请求结束时的时间，并取出上次请求结束时的时间点
		stamp := atomic.SwapInt64(&n.stamp, now)
		// td  最近两次请求结束时间的差值
		td := now - stamp
		if td < 0 {
			td = 0
		}
		w := math.Exp(float64(-td) / float64(tau))

		// 计算本次请求延迟，并保存 （从pick 到 RPC请求完成）
		start := e.Value.(int64)
		lag := now - start
		if lag < 0 {
			lag = 0
		}
		oldLag := atomic.LoadInt64(&n.lag)
		if oldLag == 0 {
			w = 0.0
		}
		// 请求延迟维度的ewma计算
		// 计算这一次的ewma值
		// lag 是本次的实际值（本次RPC请求时间）
		lag = int64(float64(oldLag)*w + float64(lag)*(1.0-w))
		atomic.StoreInt64(&n.lag, lag)

		success := uint64(1000) // error value ,if error set 1
		if di.Err != nil {
			if n.errHandler != nil && n.errHandler(di.Err) {
				success = 0
			}
			var netErr net.Error
			if errors.Is(context.DeadlineExceeded, di.Err) || errors.Is(context.Canceled, di.Err) ||
				errors.IsServiceUnavailable(di.Err) || errors.IsGatewayTimeout(di.Err) || errors.As(di.Err, &netErr) {
				success = 0
			}
		}
		// osucc 上一次的ewma值
		oldSuc := atomic.LoadUint64(&n.success)
		// 请求是否成功维度的ewma计算
		// 计算这一次的ewma值
		// success 是本次的实际值（成功为1000，失败为0）
		success = uint64(float64(oldSuc)*w + float64(success)*(1.0-w))
		atomic.StoreUint64(&n.success, success)
	}
}

// Weight is node effective weight.
func (n *Node) Weight() (weight float64) {
	weight = float64(n.health()*uint64(time.Second)) / float64(n.load())
	return
}

func (n *Node) PickElapsed() time.Duration {
	return time.Duration(time.Now().UnixNano() - atomic.LoadInt64(&n.lastPick))
}

func (n *Node) Raw() selector.Node {
	return n.Node
}
