package router

import (
	"errors"
	"net"
	"strings"
	"time"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var routerDurationBuckets = []float64{
	0.0005,
	0.001,
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
}

var (
	routerMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_router_messages_total",
		Help: "Total router messages by direction, kind, cmd, and result.",
	}, []string{"direction", "kind", "cmd", "result"})
	routerMessageDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "goone_router_message_duration_seconds",
		Help:    "Latency distribution of router send/receive operations.",
		Buckets: routerDurationBuckets,
	}, []string{"direction", "kind", "cmd"})
	routerMessageErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_router_message_errors_total",
		Help: "Total router send/receive errors.",
	}, []string{"direction", "kind", "cmd"})
	routerMessageTimeouts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_router_message_timeouts_total",
		Help: "Total router send/receive timeouts.",
	}, []string{"direction", "kind", "cmd"})
	routerMessageBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_router_message_bytes_total",
		Help: "Total router message bytes by direction, kind, and cmd.",
	}, []string{"direction", "kind", "cmd"})
	routerMessagesInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "goone_router_messages_in_flight",
		Help: "Current in-flight router send/receive operations.",
	}, []string{"direction", "kind", "cmd"})
)

func beginRouterObserve(direction, kind string, cmd uint32) func(bodyLen int, err error) {
	cmdLabel := routerCmdLabel(cmd)
	routerMessagesInFlight.WithLabelValues(direction, kind, cmdLabel).Inc()
	start := time.Now()
	return func(bodyLen int, err error) {
		routerMessagesInFlight.WithLabelValues(direction, kind, cmdLabel).Dec()
		routerMessagesTotal.WithLabelValues(direction, kind, cmdLabel, routerResultLabel(err)).Inc()
		routerMessageDuration.WithLabelValues(direction, kind, cmdLabel).Observe(time.Since(start).Seconds())
		if bodyLen > 0 {
			routerMessageBytes.WithLabelValues(direction, kind, cmdLabel).Add(float64(bodyLen))
		}
		if err != nil {
			routerMessageErrors.WithLabelValues(direction, kind, cmdLabel).Inc()
			if routerIsTimeoutErr(err) {
				routerMessageTimeouts.WithLabelValues(direction, kind, cmdLabel).Inc()
			}
		}
	}
}

func observeRouterInvalidReceive(result string, err error, bodyLen int) {
	cmdLabel := routerCmdLabel(0)
	routerMessagesTotal.WithLabelValues("receive", "bus", cmdLabel, result).Inc()
	if bodyLen > 0 {
		routerMessageBytes.WithLabelValues("receive", "bus", cmdLabel).Add(float64(bodyLen))
	}
	if err != nil {
		routerMessageErrors.WithLabelValues("receive", "bus", cmdLabel).Inc()
		if routerIsTimeoutErr(err) {
			routerMessageTimeouts.WithLabelValues("receive", "bus", cmdLabel).Inc()
		}
	}
}

func routerCmdLabel(cmd uint32) string {
	label := strings.TrimSpace(g1_protocol.CMD(cmd).String())
	if label == "" || label == "0" {
		return "UNKNOWN"
	}
	return label
}

func routerResultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	if routerIsTimeoutErr(err) {
		return "timeout"
	}
	return "error"
}

func routerIsTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}
