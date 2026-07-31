package redis

import (
	"context"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/service/runtime"
)

// Component 把一个 RedisMgr 包装成 runtime.Component，使其资源生命周期由运行期统一管理
// 。
//
// 历史缺陷：多个服务通过只有 OnStart 的 FuncComponent 初始化 Redis，Stop 不关闭，连接池在
// 进程生命周期内泄漏。Component 在 Stop 时调用 RedisMgr.Close，并返回聚合的 Close error，
// 使资源泄漏可由 Runtime 观测。
//
// 该 Component 只拥有自己的 RedisMgr，不引用服务全局变量，也不依赖其他 Component。
type Component struct {
	name string
	mgr  *RedisMgr
	conf []Config
}

// NewComponent 构造一个 Redis Component。name 为组件名（用于 Runtime 指标与日志），
// mgr 为持有的 RedisMgr（通常为服务 globals 的单例），conf 为初始化用的实例配置。
func NewComponent(name string, mgr *RedisMgr, conf []Config) *Component {
	if mgr == nil {
		mgr = NewRedisMgr()
	}
	return &Component{name: name, mgr: mgr, conf: conf}
}

// Name 实现 runtime.Component。
func (c *Component) Name() string { return c.name }

// Start 实现 runtime.Component：初始化所有 Redis 实例。失败时 RedisMgr 自身已逆序回滚
// 已成功实例（见 InitAndRun），Component 不做额外清理。
func (c *Component) Start(_ context.Context) error {
	if err := c.mgr.InitAndRun(c.conf); err != nil {
		return fmt.Errorf("redis component %q start: %w", c.name, err)
	}
	return nil
}

// Stop 实现 runtime.Component：关闭所有 Redis 实例并返回聚合 error。幂等。
func (c *Component) Stop(_ context.Context) error {
	if err := c.mgr.Close(); err != nil {
		return fmt.Errorf("redis component %q stop: %w", c.name, err)
	}
	return nil
}

// 接口断言：确保 Component 实现 runtime.Component。
var _ runtime.Component = (*Component)(nil)
