package ssrpc

import (
	"context"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

// Registrar 是把一个服务的 binding 批次注册进 Registry 的生成器函数。生成
// 器为每个服务生成 Register<Service>ToRegistry(r *Registry, srv SServer) error，其签名
// 与本类型匹配。
type Registrar func(*Registry) error

// RegistryComponentOption 配置 RegistryComponent。
type RegistryComponentOption func(*RegistryComponent)

// WithTransactionManager 注入 TransactionMgr，使 Seal 后自动把 CMD binding 绑定到
// TransactionMgr（RegisterToTransactionMgr）。bus 服务用此选项；web 服务不设。
func WithTransactionManager(mgr transaction.ITransactionMgr) RegistryComponentOption {
	return func(c *RegistryComponent) {
		c.transMgr = mgr
	}
}

// RegistryComponent 把一个或多个生成 Registrar 装配为一个 runtime.Component。
//
// Start 固定顺序：
//  1. 创建 App 实例级 Registry（每个 RegistryComponent 一个）。
//  2. 调用一个或多个生成 Registrar（Register<Service>ToRegistry）。
//  3. Registry 对完整批次校验和查重。
//  4. Seal 为只读 Dispatcher。
//  5. bus 服务把 CMD Handler 原子绑定到 TransactionMgr（RegisterToTransactionMgr 返回
//     error）。
//
// Stop 为无资源 no-op。Dispatcher() 返回已 Seal 的 Dispatcher，供 web 服务挂载 Gin/gRPC。
type RegistryComponent struct {
	name       string
	register   Registrar
	transMgr   transaction.ITransactionMgr
	dispatcher *Dispatcher
}

// NewRegistryComponent 构建一个 RegistryComponent。name 是组件名（如 "ssrpc_registry"）；
// register 是把服务 binding 注册进 Registry 的生成函数。
func NewRegistryComponent(name string, register Registrar, opts ...RegistryComponentOption) *RegistryComponent {
	c := &RegistryComponent{name: name, register: register}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Name 实现 runtime.Component（本包不依赖 runtime，方法签名匹配）。
func (c *RegistryComponent) Name() string { return c.name }

// Start 实现 runtime.Component：Register → Seal → 可选 TransactionMgr 绑定。
func (c *RegistryComponent) Start(_ context.Context) error {
	r := NewRegistry()
	if err := c.register(r); err != nil {
		return fmt.Errorf("%s: register: %w", c.name, err)
	}
	d, err := r.Seal()
	if err != nil {
		return fmt.Errorf("%s: seal: %w", c.name, err)
	}
	c.dispatcher = d
	if c.transMgr != nil {
		if err := d.RegisterToTransactionMgr(c.transMgr); err != nil {
			return fmt.Errorf("%s: bind to transaction manager: %w", c.name, err)
		}
	}
	return nil
}

// Stop 实现 runtime.Component：无资源，no-op。Dispatcher 保留供 Drain 期读。
func (c *RegistryComponent) Stop(_ context.Context) error { return nil }

// Dispatcher 返回 Start 后已 Seal 的 Dispatcher；Start 前为 nil。web 服务用它挂载
// Gin/gRPC，bus 服务一般不直接用（TransMgr 已绑定）。
func (c *RegistryComponent) Dispatcher() *Dispatcher { return c.dispatcher }
