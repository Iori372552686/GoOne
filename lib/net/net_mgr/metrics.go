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

var gatewayConnectionsDesc = prometheus.NewDesc(
	"goone_gateway_connections",
	"Current gateway connection counts by transport and kind.",
	[]string{"transport", "kind"},
	nil,
)

var (
	gatewayCollectorOnce sync.Once
	gatewaySources       sync.Map // map[any]string
)

type gatewayConnectionsCollector struct{}

func registerGatewaySource(transport string, source any) {
	if source == nil || transport == "" {
		return
	}
	gatewayCollectorOnce.Do(func() {
		prometheus.MustRegister(gatewayConnectionsCollector{})
	})
	gatewaySources.Store(source, transport)
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
	type totals struct {
		uidSessions int
		remoteAddrs int
	}

	byTransport := map[string]totals{}
	gatewaySources.Range(func(key, value any) bool {
		transport, ok := value.(string)
		if !ok || transport == "" {
			return true
		}

		snapshot := byTransport[transport]
		switch src := key.(type) {
		case *ConnTcpSvr:
			src.lock.RLock()
			snapshot.uidSessions += len(src.uidConnMap)
			snapshot.remoteAddrs += len(src.remoteAddrConnMap)
			src.lock.RUnlock()
		case *ConnWsTcpSvr:
			src.lock.RLock()
			snapshot.uidSessions += len(src.uidConnMap)
			snapshot.remoteAddrs += len(src.remoteAddrConnMap)
			src.lock.RUnlock()
		case *ConnKcpSvr:
			src.lock.RLock()
			snapshot.uidSessions += len(src.uidConnMap)
			snapshot.remoteAddrs += len(src.remoteAddrConnMap)
			src.lock.RUnlock()
		}
		byTransport[transport] = snapshot
		return true
	})

	for transport, snapshot := range byTransport {
		ch <- prometheus.MustNewConstMetric(gatewayConnectionsDesc, prometheus.GaugeValue, float64(snapshot.uidSessions), transport, "uid_sessions")
		ch <- prometheus.MustNewConstMetric(gatewayConnectionsDesc, prometheus.GaugeValue, float64(snapshot.remoteAddrs), transport, "remote_addrs")
	}
}
