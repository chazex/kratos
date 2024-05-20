package http

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/go-kratos/aegis/subset"
	"github.com/go-kratos/kratos/v2/internal/endpoint"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/selector"
)

// Target is resolver target
type Target struct {
	Scheme    string
	Authority string
	Endpoint  string
}

func parseTarget(endpoint string, insecure bool) (*Target, error) {
	// 拼接上协议 http:// 或者 https://
	if !strings.Contains(endpoint, "://") {
		if insecure {
			endpoint = "http://" + endpoint
		} else {
			endpoint = "https://" + endpoint
		}
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	target := &Target{Scheme: u.Scheme, Authority: u.Host}
	if len(u.Path) > 1 {
		target.Endpoint = u.Path[1:]
	}
	return target, nil
}

type resolver struct {
	rebalancer selector.Rebalancer

	target      *Target
	watcher     registry.Watcher
	selecterKey string
	// 对服务发现的Host列表，做subset。
	// 如果设置为0， 则不做subset
	subsetSize int

	insecure bool
}

func newResolver(ctx context.Context, discovery registry.Discovery, target *Target,
	rebalancer selector.Rebalancer, block, insecure bool, subsetSize int,
) (*resolver, error) {
	// 服务发现的watcher
	// this is new resovler
	watcher, err := discovery.Watch(ctx, target.Endpoint)
	if err != nil {
		return nil, err
	}
	r := &resolver{
		target:      target,
		watcher:     watcher,
		rebalancer:  rebalancer,
		insecure:    insecure,
		selecterKey: uuid.New().String(),
		subsetSize:  subsetSize,
	}
	// block是表示阻塞，这个场景是当app刚启动时，依赖的服务列表为空，所以不能异步获取服务列表，容易导致app开始接受请求，但是依赖的服务列表没准备好，而出现错误的情况
	// 所以需要阻塞式的获取服务列表，直到成功
	if block {
		done := make(chan error, 1)
		go func() {
			for {
				// 节点列表有变化
				services, err := watcher.Next()
				if err != nil {
					done <- err
					return
				}
				// 更新服务节点， 成功后协程退出
				if r.update(services) {
					done <- nil
					return
				}
			}
		}()
		select {
		case err := <-done:
			if err != nil {
				stopErr := watcher.Stop()
				if stopErr != nil {
					log.Errorf("failed to http client watch stop: %v, error: %+v", target, stopErr)
				}
				return nil, err
			}
		case <-ctx.Done():
			log.Errorf("http client watch service %v reaching context deadline!", target)
			stopErr := watcher.Stop()
			if stopErr != nil {
				log.Errorf("failed to http client watch stop: %v, error: %+v", target, stopErr)
			}
			return nil, ctx.Err()
		}
	}
	// 启动协程
	go func() {
		for {
			// watcher.Next() 是阻塞函数，当服务节点列表发生变化时，才会返回
			services, err := watcher.Next()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Errorf("http client watch service %v got unexpected error:=%v", target, err)
				time.Sleep(time.Second)
				continue
			}
			// 更新服务节点列表
			r.update(services)
		}
	}()
	return r, nil
}

// 将服务发现得到的节点实例列表([]*registry.ServiceInstance)，转换为负载均衡的Node列表，并更新到负载均衡器内部。
// 负载均衡器会根据其内部的节点信息，做负载均衡
func (r *resolver) update(services []*registry.ServiceInstance) bool {
	// 服务发现获取的 Host列表
	filtered := make([]*registry.ServiceInstance, 0, len(services))
	for _, ins := range services {
		// 获取节点的Host
		ept, err := endpoint.ParseEndpoint(ins.Endpoints, endpoint.Scheme("http", !r.insecure))
		if err != nil {
			log.Errorf("Failed to parse (%v) discovery endpoint: %v error %v", r.target, ins.Endpoints, err)
			continue
		}
		if ept == "" {
			continue
		}
		filtered = append(filtered, ins)
	}
	if r.subsetSize != 0 {
		// 做subset
		filtered = subset.Subset(r.selecterKey, filtered, r.subsetSize)
	}
	nodes := make([]selector.Node, 0, len(filtered))
	for _, ins := range filtered {
		ept, _ := endpoint.ParseEndpoint(ins.Endpoints, endpoint.Scheme("http", !r.insecure))
		// 将服务发现得到的ServiceInstance， 转换为负载均衡的node
		nodes = append(nodes, selector.NewNode("http", ept, ins))
	}

	if len(nodes) == 0 {
		log.Warnf("[http resolver]Zero endpoint found,refused to write,set: %s ins: %v", r.target.Endpoint, nodes)
		return false
	}
	// 更新负载均衡器的内部服务节点。
	r.rebalancer.Apply(nodes)
	return true
}

func (r *resolver) Close() error {
	return r.watcher.Stop()
}
