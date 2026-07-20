package bus

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Driver 是一个显式的、链接期的 bus driver 描述符。每个 driver 包
// （rabbitmq、nats、kafka、nsq、rocketmq）导出一个 Driver() 函数返回此种描述符；
// 应用只链接所需 driver 并把它们注册到 DriverRegistry，取代遗留的 blank-import /
// func init 自注册。
//
// 一个 Driver 携带：
//   - Name：与 ParseAddr 匹配的 implType 字符串（如 "rabbitmq"）。
//   - Ctor：构建具体 IBus 的 BusCtor。
//
// 使用显式 Driver 让服务二进制可以完全省略未使用的 MQ SDK（更小的二进制、更小的
// 漏洞面），同时接线后仍保留标准 bus 生命周期。
type Driver struct {
	Name string
	Ctor BusCtor
}

// DriverRegistry 是已选 bus driver 的 app 实例级注册表。不同于包级全局的 busCtors
// map（由各 driver 的 init 填充），DriverRegistry 每 App 创建一份，且只包含应用显式
// 注册的 driver。CreateBusFromRegistry 在此查找 driver，故未链接的 driver 会产生一
// 个清晰错误并列出当前可用项。
//
// DriverRegistry 在装配后（注册在装配期单线程进行；查找在 Start 期发生）可并发使
// 用。
type DriverRegistry struct {
	mu      sync.RWMutex
	drivers map[string]BusCtor
}

// NewDriverRegistry 构建一个空注册表。
func NewDriverRegistry() *DriverRegistry {
	return &DriverRegistry{drivers: make(map[string]BusCtor)}
}

// Register 添加一个 driver。重名、空名与 nil ctor 被拒绝，使装配错误在启动期而非
// 运行期暴露。
func (r *DriverRegistry) Register(d Driver) error {
	if r == nil {
		return ErrNilDriverRegistry
	}
	name := strings.TrimSpace(d.Name)
	if name == "" {
		return ErrEmptyDriverName
	}
	if d.Ctor == nil {
		return ErrNilDriverCtor
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.drivers[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateDriver, name)
	}
	r.drivers[name] = d.Ctor
	return nil
}

// MustRegister 在 Register 出错时 panic；仅在装配代码中使用。
func (r *DriverRegistry) MustRegister(d Driver) {
	if err := r.Register(d); err != nil {
		panic(err)
	}
}

// RegisterAll 原子地注册多个 driver：若任一失败，registry 保持不变。
func (r *DriverRegistry) RegisterAll(drivers ...Driver) error {
	if r == nil {
		return ErrNilDriverRegistry
	}
	// 先校验整批，不改动状态。
	seen := make(map[string]struct{}, len(drivers))
	for i, d := range drivers {
		name := strings.TrimSpace(d.Name)
		if name == "" {
			return fmt.Errorf("driver %d: %w", i, ErrEmptyDriverName)
		}
		if d.Ctor == nil {
			return fmt.Errorf("driver %s: %w", name, ErrNilDriverCtor)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("driver %s: %w", name, ErrDuplicateDriver)
		}
		if r.has(name) {
			return fmt.Errorf("driver %s: %w", name, ErrDuplicateDriver)
		}
		seen[name] = struct{}{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range drivers {
		r.drivers[strings.TrimSpace(d.Name)] = d.Ctor
	}
	return nil
}

// Names 返回已注册 driver 名的有序（排序）列表（用于诊断）。
func (r *DriverRegistry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.drivers))
	for name := range r.drivers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// has 上报 name 是否已注册；调用方可持有 r.mu 也可不持有。
func (r *DriverRegistry) has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.drivers[name]
	return ok
}

// CreateBus 仅用显式注册的 driver 构建一个 bus 实例。implType 为空时默认为
// "rabbitmq"（与遗留默认一致）。若请求的 driver 未注册，错误列出可用名字，使运维
// 知道链接了哪些 driver。
//
// 不同于包级全局的 CreateBus，它绝不回退到 init 注册的 driver：未链接的 driver 是
// 配置错误，而非静默成功。
func (r *DriverRegistry) CreateBus(selfBusId uint32, onRecvMsg MsgHandler, addr string) (IBus, error) {
	if r == nil {
		return nil, ErrNilDriverRegistry
	}
	implType, cfg, err := ParseAddr(addr)
	if err != nil {
		return nil, err
	}
	if implType == "" {
		implType = "rabbitmq"
	}
	r.mu.RLock()
	ctor, ok := r.drivers[implType]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("bus driver %q not registered (available: %v); link the driver package and register it, or fix bus_mq_addr", implType, r.Names())
	}
	return ctor(selfBusId, onRecvMsg, cfg)
}

// FromGlobal 把遗留的包级全局 busCtors 收纳进一个 DriverRegistry。它是过渡期的桥
// 接：仍 blank-import driver/all 的服务可从全局已注册 ctor 构造一个 DriverRegistry，
// 而无需改动其 import 图。新服务应改为注册显式 Driver 描述符。
func FromGlobal() *DriverRegistry {
	r := NewDriverRegistry()
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, ctor := range busCtors {
		r.drivers[name] = ctor
	}
	return r
}

// driver registry 的哨兵错误。
var (
	// ErrNilDriverRegistry 在 nil registry 上调用方法时返回。
	ErrNilDriverRegistry = errors.New("bus: nil driver registry")
	// ErrEmptyDriverName 在 driver 名为空时返回。
	ErrEmptyDriverName = errors.New("bus: driver 名为空")
	// ErrNilDriverCtor 在 driver ctor 为 nil 时返回。
	ErrNilDriverCtor = errors.New("bus: nil driver ctor")
	// ErrDuplicateDriver 在 driver 名已注册时返回。
	ErrDuplicateDriver = errors.New("bus: 重复的 driver")
)
