package registry

import (
	"context"
	"fmt"
	"sort"
)

// Registrar 服务注册接口 （server用）

// Registrar is service registrar.
type Registrar interface {
	// Register 注册方法
	// Register the registration.
	Register(ctx context.Context, service *ServiceInstance) error
	// Deregister 反注册方法
	// Deregister the registration.
	Deregister(ctx context.Context, service *ServiceInstance) error
}

// Discovery 服务发现接口（client用）

// Discovery is service discovery.
type Discovery interface {
	// GetService 根据服务名获取服务节点列表
	// GetService return the service instances in memory according to the service name.
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Watch 监听某一个服务名的节点变化情况。
	// Watch creates a watcher according to the service name.
	Watch(ctx context.Context, serviceName string) (Watcher, error)
}

// Watcher 接口是配合服务发现接口(Discovery)来一起使用的。

// Watcher is service watcher.
type Watcher interface {
	// Next returns services in the following two cases:
	// 1.the first time to watch and the service instance list is not empty.
	// 2.any service instance changes found.
	// if the above two conditions are not met, it will block until context deadline exceeded or canceled
	// 这里叫做Next()方法，感觉稍微有点容易混淆，因为Next，一般用来表示获取下一个，我们很容易理解成获取下一个节点。
	// 这个函数实际上是，阻塞监听服务节点变更事件，有变化，则返回新的节点列表
	//	当首次调用的时候，会从注册中心（如etcd）查询所有节点列表并返回
	//  当不是首次调用的时候，会检查etcd是否发生变更(通过etcd的变更通知)，如果有变更，则会拉取新的服务列表，并返回。 如果没有变更，会阻塞。
	// 由于没有变更会阻塞，所以这个函数一般是在一个协程中异步调用，并且是在一个循环中
	Next() ([]*ServiceInstance, error)
	// Stop close the watcher.
	Stop() error
}

// ServiceInstance 服务注册的实例

// ServiceInstance is an instance of a service in a discovery system.
type ServiceInstance struct {
	// ID is the unique instance ID as registered.
	ID string `json:"id"`
	// Name is the service name as registered.
	Name string `json:"name"`
	// Version is the version of the compiled.
	Version string `json:"version"`
	// Metadata is the kv pair metadata associated with the service instance.
	Metadata map[string]string `json:"metadata"`
	// Endpoints are endpoint addresses of the service instance.
	// schema:
	//   http://127.0.0.1:8000?isSecure=false
	//   grpc://127.0.0.1:9000?isSecure=false
	Endpoints []string `json:"endpoints"` // 服务对应的所有可用实例
}

func (i *ServiceInstance) String() string {
	return fmt.Sprintf("%s-%s", i.Name, i.ID)
}

// Equal returns whether i and o are equivalent.
func (i *ServiceInstance) Equal(o interface{}) bool {
	if i == nil && o == nil {
		return true
	}

	if i == nil || o == nil {
		return false
	}

	t, ok := o.(*ServiceInstance)
	if !ok {
		return false
	}

	if len(i.Endpoints) != len(t.Endpoints) {
		return false
	}

	sort.Strings(i.Endpoints)
	sort.Strings(t.Endpoints)
	for j := 0; j < len(i.Endpoints); j++ {
		if i.Endpoints[j] != t.Endpoints[j] {
			return false
		}
	}

	if len(i.Metadata) != len(t.Metadata) {
		return false
	}

	for k, v := range i.Metadata {
		if v != t.Metadata[k] {
			return false
		}
	}

	return i.ID == t.ID && i.Name == t.Name && i.Version == t.Version
}
