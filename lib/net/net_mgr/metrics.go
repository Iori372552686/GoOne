package net_mgr

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var gatewayEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "goone_gateway_events_total",
	Help: "Total gateway connection lifecycle and IO events by transport.",
}, []string{"transport", "event"})

// V3-P1-02：gateway_connections 改为全局指标（不再按 transport 拆分）。
// SessionHub 是三传输合一的会话状态拥有者，无法按 tcp/ws/kcp 拆分会话计数；
// 单路径化后从 hub 的 ActiveSessions/ActiveConnections 取全局值。
var gatewayConnectionsDesc = prometheus.NewDesc(
	"goone_gateway_connections",
	"Current gateway connection counts (global across transports).",
	[]string{"kind"},
	nil,
)

var (
	gatewayCollectorOnce sync.Once
	// gatewayHubSource 是共享 SessionHub，供 collector 取全局连接/会话计数。
	// 由 SetHub 在注入 hub 时设置（单路径化后 hub 是唯一事实源）。
	gatewayHubSource *SessionHub
	gatewayHubMu     sync.RWMutex
)

type gatewayConnectionsCollector struct{}

// registerGatewaySource 保留 transport 标签枚举用途（gatewayEventsTotal 仍按 transport
// 上报事件）。单路径化后连接计数不再依赖各传输实例。
func registerGatewaySource(transport string, source any) {
	_ = source // 单路径化后连接计数改走 hub，source 仅保留参数兼容。
	_ = transport
}

// registerGatewayHub 注册共享 SessionHub 作为连接计数源（V3-P1-02）。
func registerGatewayHub(hub *SessionHub) {
	if hub == nil {
		return
	}
	gatewayCollectorOnce.Do(func() {
		prometheus.MustRegister(gatewayConnectionsCollector{})
	})
	gatewayHubMu.Lock()
	gatewayHubSource = hub
	gatewayHubMu.Unlock()
}

func observeGatewayEvent(transport, event string) {
	if transport == "" || event == "" {
		return
	}
	gatewayEventsTotal.WithLabelValues(transport, event).Inc()
}

func (gatewayConnectionsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- gatewayConnectionsDesc
}

func (gatewayConnectionsCollector) Collect(ch chan<- prometheus.Metric) {
	gatewayHubMu.RLock()
	hub := gatewayHubSource
	gatewayHubMu.RUnlock()
	if hub == nil {
		return
	}
	// 全局会话数（已绑定 UID 的逻辑会话）与连接数（底层连接，含未认证）。
	ch <- prometheus.MustNewConstMetric(gatewayConnectionsDesc, prometheus.GaugeValue,
		float64(hub.ActiveSessions()), "uid_sessions")
	ch <- prometheus.MustNewConstMetric(gatewayConnectionsDesc, prometheus.GaugeValue,
		float64(hub.ActiveConnections()), "connections")
}
