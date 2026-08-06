// Package scheduler —— datetime 缓存刷新预设。
//
// datetime 是被 logger/xorm/net 等基础库直接 import 的叶子包，自身不能反向 import
// scheduler（否则形成 datetime→scheduler→logger→datetime 的循环）。因此 datetime
// 的周期刷新由 scheduler 侧主动驱动：本文件提供一个开箱即用的 Task 工厂，各 service
// 在 app.Register 列表注册一行即可，无需手写 tick 逻辑。
package datetime

import (
	"context"

	"github.com/Iori372552686/GoOne/lib/service/scheduler"
)

// DefaultDateTimeTick 返回一个驱动 datetime 缓存刷新的 Task，作为
// runtime.Component 注册到 App。
//
//   - 周期：datetime.TickInterval()（默认 100ms，可通过 datetime.SetTickInterval
//     在 LoadConfig 阶段覆盖）。
//   - Inline=true：在 loop 内同步调用 datetime.Tick()，零额外 goroutine。datetime.Tick
//     是纳秒级单次 atomic 存储，绝无阻塞或重叠风险，正适合 Inline。
//

// 应注册在所有依赖 datetime 的组件之前（如 logger/xorm/tcp_server 启动时即读时间）。
func DefaultDateTimeTick() *scheduler.Task {
	t := scheduler.New(
		"datetime_tick",
		TickInterval(),
		func(_ context.Context) error {
			Tick()
			return nil
		},
	)
	t.Inline = true
	return t
}
