package ssrpc

import (
	"strings"
	"sync"
	"time"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	defaultMetricsRecorder MetricsRecorder = promMetricsRecorder{}
	ssrpcMetricsOnce       sync.Once

	ssrpcRequestTotal     *prometheus.CounterVec
	ssrpcRequestDuration  *prometheus.HistogramVec
	ssrpcRequestErrors    *prometheus.CounterVec
	ssrpcRequestTimeouts  *prometheus.CounterVec
	ssrpcRequestsInFlight *prometheus.GaugeVec
)

var ssrpcDurationBuckets = []float64{
	0.001,
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
	2.5,
	5,
}

// MetricsRecorder is an optional hook to record ssrpc handler metrics.
type MetricsRecorder interface {
	Observe(cmd g1_protocol.CMD, cost time.Duration, code g1_protocol.ErrorCode)
}

// MetricsRecorderWithContext is an optional richer hook for recorders that want
// access to session, method, and transport metadata without breaking the older
// MetricsRecorder interface.
type MetricsRecorderWithContext interface {
	ObserveWithContext(ctx *Context, cost time.Duration, code g1_protocol.ErrorCode)
}

type metricsRecorderLifecycle interface {
	StartObserve(ctx *Context) func()
}

type promMetricsRecorder struct{}

// DefaultMetricsRecorder returns the built-in Prometheus recorder used by the
// default middleware chain.
func DefaultMetricsRecorder() MetricsRecorder {
	initSSRPCMetrics()
	return defaultMetricsRecorder
}

func (promMetricsRecorder) StartObserve(ctx *Context) func() {
	initSSRPCMetrics()
	transport, method, cmd := ssrpcMetricLabels(ctx)
	ssrpcRequestsInFlight.WithLabelValues(transport, method, cmd).Inc()
	return func() {
		ssrpcRequestsInFlight.WithLabelValues(transport, method, cmd).Dec()
	}
}

func (promMetricsRecorder) Observe(cmd g1_protocol.CMD, cost time.Duration, code g1_protocol.ErrorCode) {
	initSSRPCMetrics()
	observeSSRPCMetrics("", "", cmd, cost, code)
}

func (promMetricsRecorder) ObserveWithContext(ctx *Context, cost time.Duration, code g1_protocol.ErrorCode) {
	initSSRPCMetrics()
	transport, method, _ := ssrpcMetricLabels(ctx)
	observeSSRPCMetrics(transport, method, ctxCmd(ctx), cost, code)
}

func initSSRPCMetrics() {
	ssrpcMetricsOnce.Do(func() {
		ssrpcRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "goone_ssrpc_requests_total",
			Help: "Total ssrpc requests handled by transport, method, cmd, and code.",
		}, []string{"transport", "method", "cmd", "code"})
		ssrpcRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "goone_ssrpc_request_duration_seconds",
			Help:    "Latency distribution of ssrpc requests.",
			Buckets: ssrpcDurationBuckets,
		}, []string{"transport", "method", "cmd"})
		ssrpcRequestErrors = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "goone_ssrpc_request_errors_total",
			Help: "Total non-OK ssrpc requests.",
		}, []string{"transport", "method", "cmd", "code"})
		ssrpcRequestTimeouts = promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "goone_ssrpc_request_timeouts_total",
			Help: "Total timed out ssrpc requests.",
		}, []string{"transport", "method", "cmd"})
		ssrpcRequestsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "goone_ssrpc_requests_in_flight",
			Help: "Current in-flight ssrpc requests.",
		}, []string{"transport", "method", "cmd"})
	})
}

func observeSSRPCMetrics(transport, method string, cmd g1_protocol.CMD, cost time.Duration, code g1_protocol.ErrorCode) {
	if transport == "" {
		transport = "unknown"
	}
	if method == "" {
		method = "unknown"
	}
	cmdLabel := ssrpcCmdLabel(cmd)
	codeLabel := strings.TrimSpace(code.String())
	if codeLabel == "" || codeLabel == "0" {
		codeLabel = g1_protocol.ErrorCode_ERR_OK.String()
	}

	ssrpcRequestTotal.WithLabelValues(transport, method, cmdLabel, codeLabel).Inc()
	ssrpcRequestDuration.WithLabelValues(transport, method, cmdLabel).Observe(cost.Seconds())
	if code != g1_protocol.ErrorCode_ERR_OK && code != g1_protocol.ErrorCode_ERR_SUCESS {
		ssrpcRequestErrors.WithLabelValues(transport, method, cmdLabel, codeLabel).Inc()
	}
	if code == g1_protocol.ErrorCode_ERR_TIMEOUT {
		ssrpcRequestTimeouts.WithLabelValues(transport, method, cmdLabel).Inc()
	}
}

func ssrpcMetricLabels(ctx *Context) (transport, method, cmd string) {
	transport = "unknown"
	method = "unknown"
	cmd = ssrpcCmdLabel(0)
	if ctx == nil {
		return transport, method, cmd
	}

	if ctx.Transport != "" {
		transport = string(ctx.Transport)
	}
	if strings.TrimSpace(ctx.Method) != "" {
		method = strings.TrimSpace(ctx.Method)
	}
	cmd = ssrpcCmdLabel(ctx.Cmd)
	return transport, method, cmd
}

func ctxCmd(ctx *Context) g1_protocol.CMD {
	if ctx == nil {
		return 0
	}
	return ctx.Cmd
}

func ssrpcCmdLabel(cmd g1_protocol.CMD) string {
	label := strings.TrimSpace(cmd.String())
	if label == "" || label == "0" {
		return "UNKNOWN"
	}
	return label
}

// Metrics creates a middleware that records duration + final error code.
func Metrics(rec MetricsRecorder) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if rec == nil {
				return next(ctx, req)
			}

			var finish func()
			if lifecycle, ok := any(rec).(metricsRecorderLifecycle); ok {
				finish = lifecycle.StartObserve(ctx)
			}
			if finish != nil {
				defer finish()
			}

			start := time.Now()
			rsp, err := next(ctx, req)
			code := ToErrorCode(err)
			cost := time.Since(start)
			if rich, ok := any(rec).(MetricsRecorderWithContext); ok {
				rich.ObserveWithContext(ctx, cost, code)
				return rsp, err
			}
			if ctx != nil {
				rec.Observe(ctx.Cmd, cost, code)
			} else {
				rec.Observe(0, cost, code)
			}
			return rsp, err
		}
	}
}
