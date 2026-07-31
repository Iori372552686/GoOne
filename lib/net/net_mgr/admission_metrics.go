package net_mgr

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// admission（过载保护）决策指标。与现有 gateway/ssrpc 指标一致使用 promauto，
// 自动出现在 /metrics。
var (
	admissionDecisionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_admission_decisions_total",
		Help: "Admission decisions by gate and outcome (V3-P1-01).",
	}, []string{"gate", "decision", "reason"})
	// decision ∈ {admit, reject, shadow_reject}
	// gate   ∈ {connection, login}
)

func recordAdmission(gate, decision, reason string) {
	admissionDecisionsTotal.WithLabelValues(gate, decision, reason).Inc()
}
