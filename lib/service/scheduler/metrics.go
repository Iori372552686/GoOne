package scheduler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// 调度器可观测性指标。
//
//   - goone_task_duration_seconds：Task 单次执行耗时（按 task 名 label）。
//   - goone_task_skipped_total：Task 因 NonOverlap 被跳过的次数。
var (
	taskDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "goone",
		Subsystem: "task",
		Name:      "duration_seconds",
		Help:      "Single Task run duration in seconds.",
		Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
	}, []string{"task"})

	taskSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "goone",
		Subsystem: "task",
		Name:      "skipped_total",
		Help:      "Total Task invocations skipped to NonOverlap.",
	}, []string{"task"})
)

// observeTaskRun 上报一次 Task 执行耗时。
func observeTaskRun(name string, seconds float64) {
	taskDuration.WithLabelValues(name).Observe(seconds)
}

// incTaskSkipped 上报一次 NonOverlap 跳过。
func incTaskSkipped(name string) {
	taskSkipped.WithLabelValues(name).Inc()
}

var _ prometheus.Gauge = nil
