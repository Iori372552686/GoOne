package gormdb

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	gormPingTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_gorm_ping_total", Help: "Total GORM-backed database ping attempts.",
	}, []string{"db", "result"})
	gormPingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "goone_gorm_ping_duration_seconds", Help: "Latency of GORM-backed database ping calls.",
	}, []string{"db"})
	gormPingErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_gorm_ping_errors_total", Help: "Total GORM-backed database ping errors.",
	}, []string{"db"})
	gormPoolConnections = prometheus.NewDesc(
		"goone_gorm_pool_connections", "Current GORM-backed SQL pool connections.",
		[]string{"db", "role", "state"}, nil,
	)
	gormPoolWaitCount = prometheus.NewDesc(
		"goone_gorm_pool_wait_count_total", "Total GORM-backed SQL pool waits.",
		[]string{"db", "role"}, nil,
	)
	gormPoolWaitDuration = prometheus.NewDesc(
		"goone_gorm_pool_wait_duration_seconds_total", "Total GORM-backed SQL pool wait duration.",
		[]string{"db", "role"}, nil,
	)
)

var (
	gormCollectorOnce sync.Once
	gormRegistry      sync.Map // map[string]*instance
)

type gormPoolCollector struct{}

func registerGormMetrics(name string, entry *instance) {
	gormCollectorOnce.Do(func() { prometheus.MustRegister(gormPoolCollector{}) })
	gormRegistry.Store(name, entry)
}

func unregisterGormMetrics(name string) { gormRegistry.Delete(name) }

func (gormPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- gormPoolConnections
	ch <- gormPoolWaitCount
	ch <- gormPoolWaitDuration
}

func (gormPoolCollector) Collect(ch chan<- prometheus.Metric) {
	gormRegistry.Range(func(key, value any) bool {
		name, nameOK := key.(string)
		entry, entryOK := value.(*instance)
		if !nameOK || !entryOK || entry == nil {
			return true
		}
		for _, tracked := range entry.pools {
			stats := tracked.db.Stats()
			ch <- prometheus.MustNewConstMetric(gormPoolConnections, prometheus.GaugeValue, float64(stats.OpenConnections), name, tracked.role, "open")
			ch <- prometheus.MustNewConstMetric(gormPoolConnections, prometheus.GaugeValue, float64(stats.InUse), name, tracked.role, "in_use")
			ch <- prometheus.MustNewConstMetric(gormPoolConnections, prometheus.GaugeValue, float64(stats.Idle), name, tracked.role, "idle")
			ch <- prometheus.MustNewConstMetric(gormPoolWaitCount, prometheus.CounterValue, float64(stats.WaitCount), name, tracked.role)
			ch <- prometheus.MustNewConstMetric(gormPoolWaitDuration, prometheus.CounterValue, stats.WaitDuration.Seconds(), name, tracked.role)
		}
		return true
	})
}

func beginGormPingObserve(name string) func(error) {
	if name == "" {
		name = "default"
	}
	start := time.Now()
	return func(err error) {
		result := "ok"
		if err != nil {
			result = "error"
			if isTimeout(err) {
				result = "timeout"
			}
			gormPingErrors.WithLabelValues(name).Inc()
		}
		gormPingTotal.WithLabelValues(name, result).Inc()
		gormPingDuration.WithLabelValues(name).Observe(time.Since(start).Seconds())
	}
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "timeout")
}
