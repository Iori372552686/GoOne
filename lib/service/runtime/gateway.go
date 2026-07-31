package runtime

import (
	"context"
)

// GatewayServer 是 runtime 层定义的网关生命周期契约。它把“接流、排空、停止”三个
// 阶段与活跃计数显式化，使一个网关组件能被 App 的 Draining/Stopping 阶段统一驱动。
//
// 与 lib/net/net_mgr.GatewayServer（session-facing：Send/Kick/GetClient）正交：后者
// 描述会话面操作，本接口描述生命周期面操作。一个具体网关实现通常同时实现两者。
//
// 本契约作为 runtime 层的新约定落地；现有 tcp/ws/kcp/gnet 传输在迁移
// 时补齐 listener 字段与 Stop 能力，并实现本接口。在此之前的迁移期内，实现本接口
// 的新网关代码可立即获得统一排空。
type GatewayServer interface {
	// ActiveConnections 返回底层已建立且未关闭的连接数。
	ActiveConnections() int64
	// ActiveSessions 返回已绑定 UID 的逻辑会话数。重复绑定不重复计数；OnClose 只
	// 减一次。
	ActiveSessions() int64
}

// GatewayQuiescer 由能“停止接新连接/新会话”的网关实现。Quiesce 在 Drain 之前执行，
// 典型行为：
//   - TCP/KCP：关闭 listener，保留既有连接；
//   - WS：停止新 Upgrade；
//   - gnet：OnOpened 拒绝新连接；
//   - 已存在但未认证的连接不能再完成新登录绑定；
//   - readyz 此刻已返回 503。
type GatewayQuiescer interface {
	// QuiesceGateway 停止接收新连接与新会话。返回的 error 只记录、不阻断后续 Drain。
	QuiesceGateway(ctx context.Context) error
}

// GatewayDrainer 由“存在活跃会话需要等待自然退出”的网关实现。Drain 在 Quiesce 之后、
// Stop 之前执行，受 App 排空超时约束：
//   - 已有会话可完成在途事务并主动退出；
//   - 等待 ActiveSessions 归零（由 SessionTracker 的状态变更通知驱动，不用轮询）；
//   - 超时后由 Stop 强制关闭残留连接。
type GatewayDrainer interface {
	// DrainSessions 等待活跃会话归零或 ctx 超时。返回的 error 只记录、不阻断 Stop。
	DrainSessions(ctx context.Context) error
}

// GatewayLifecycle 是上述三接口的便利聚合，供需要同时实现三阶段的网关组件嵌入。
// 实现方既可分别实现 GatewayQuiescer/GatewayDrainer，也可直接实现本接口。
type GatewayLifecycle interface {
	GatewayServer
	GatewayQuiescer
	GatewayDrainer
}
