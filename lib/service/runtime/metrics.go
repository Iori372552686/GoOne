package runtime

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 运行时生命周期可观测性指标（roadmap P1-03）。
//
// 命名空间统一为 goone，按领域分子系统：lifecycle（状态机）、component（组件启停）、
// drain（排空）、task（调度器）、config（重载）。这些指标由 App / ComponentTracker /
// scheduler / appconfig 在对应事件点上报，经 admin /metrics 暴露。
//
// 指标清单（与 docs/observability 对齐）：
//   - goone_lifecycle_state：当前状态（gauge，按状态名 label）。
//   - goone_component_start_duration_seconds：组件 Start 耗时。
//   - goone_component_start_failures_total：组件 Start 失败次数。
//   - goone_drain_duration_seconds：排空阶段耗时。
//   - goone_drain_timeouts_total：排空超时次数。
//   - goone_task_duration_seconds：Task 单次执行耗时。
//   - goone_task_skipped_total：Task 因 NonOverlap 跳过次数。
//   - goone_config_reload_total：配置重载次数（按结果 label）。

var (
	lifecycleStateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "goone",
		Subsystem: "lifecycle",
		Name:      "state",
		Help:      "Current lifecycle state; 1 for the active state, 0 otherwise (label: state).",
	}, []string{"state"})

	componentStartDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "goone",
		Subsystem: "component",
		Name:      "start_duration_seconds",
		Help:      "Component Start duration in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"component"})

	componentStartFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "goone",
		Subsystem: "component",
		Name:      "start_failures_total",
		Help:      "Total component Start failures.",
	}, []string{"component"})

	drainDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "goone",
		Subsystem: "drain",
		Name:      "duration_seconds",
		Help:      "Drain phase duration in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.05, 2, 12),
	})

	drainTimeouts = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "goone",
		Subsystem: "drain",
		Name:      "timeouts_total",
		Help:      "Total number of drain phases that timed out.",
	})

	configReloadTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "goone",
		Subsystem: "config",
		Name:      "reload_total",
		Help:      "Configuration reload attempts (label: result=applied|restart_required|failed).",
	}, []string{"result"})
)

// observeComponentStart 上报组件 Start 耗时。
func observeComponentStart(name string, seconds float64) {
	componentStartDuration.WithLabelValues(name).Observe(seconds)
}

// incComponentStartFailure 上报组件 Start 失败。
func incComponentStartFailure(name string) {
	componentStartFailures.WithLabelValues(name).Inc()
}

// setLifecycleState 把状态机指标置为当前状态：活动状态=1，其余=0。
// 调用方在状态转换时调用，传入新旧状态以翻转 gauge。
func setLifecycleState(from, to State) {
	lifecycleStateGauge.WithLabelValues(string(from)).Set(0)
	lifecycleStateGauge.WithLabelValues(string(to)).Set(1)
}

// observeDrain 上报排空阶段耗时；timedOut 为 true 时增加超时计数。
func observeDrain(seconds float64, timedOut bool) {
	drainDuration.Observe(seconds)
	if timedOut {
		drainTimeouts.Inc()
	}
}

// incConfigReload 上报一次配置重载结果。
func incConfigReload(result string) {
	configReloadTotal.WithLabelValues(result).Inc()
}

// 保留 prometheus import（gauge 类型引用）。
var _ prometheus.Gauge = nil
