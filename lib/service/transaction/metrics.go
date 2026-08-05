package transaction

import (
	"strings"
	"sync"
	"time"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var transactionDurationBuckets = []float64{
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

var (
	transactionPacketsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_transaction_packets_total",
		Help: "Total packets observed by the transaction manager, grouped by phase/result/cmd.",
	}, []string{"phase", "cmd", "result"})
	transactionHandlerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "goone_transaction_handler_duration_seconds",
		Help:    "Latency distribution of transaction handler execution.",
		Buckets: transactionDurationBuckets,
	}, []string{"cmd"})
	transactionHandlerErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_transaction_handler_errors_total",
		Help: "Total non-OK transaction handler completions.",
	}, []string{"cmd", "code"})
	transactionTimeouts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_transaction_timeouts_total",
		Help: "Total transaction-layer timeouts.",
	}, []string{"stage", "cmd"})
	transactionActiveGauge = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goone_transaction_active_transactions",
		Help: "Current active transactions in this process.",
	}, func() float64 {
		return sumTransactionMetric(func(m *TransactionMgr) float64 {
			return float64(m.activeTransactions.Load())
		})
	})
	transactionPendingGauge = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goone_transaction_pending_packets",
		Help: "Current pending packets waiting behind serial keys.",
	}, func() float64 {
		return sumTransactionMetric(func(m *TransactionMgr) float64 {
			return float64(m.pendingPackets.Load())
		})
	})
	transactionDispatchQueueGauge = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goone_transaction_dispatch_queue_length",
		Help: "Current total length of transaction shard input queues.",
	}, func() float64 {
		return sumTransactionQueueMetric(func(shard *transactionShard) int {
			return len(shard.chanInPacket)
		})
	})
	transactionReturnQueueGauge = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "goone_transaction_return_queue_length",
		Help: "Current total length of transaction shard completion queues.",
	}, func() float64 {
		return sumTransactionQueueMetric(func(shard *transactionShard) int {
			return len(shard.chanTransRet)
		})
	})
)

var registeredTransactionMgrs sync.Map

func registerTransactionMgr(m *TransactionMgr) {
	if m == nil {
		return
	}
	registeredTransactionMgrs.Store(m, struct{}{})
}

func unregisterTransactionMgr(m *TransactionMgr) {
	if m == nil {
		return
	}
	registeredTransactionMgrs.Delete(m)
}

func observeTransactionPacket(phase string, cmd uint32, result string) {
	transactionPacketsTotal.WithLabelValues(phase, transactionCmdLabel(cmd), result).Inc()
}

func observeTransactionHandler(cmd uint32, code g1_protocol.ErrorCode, cost time.Duration) {
	cmdLabel := transactionCmdLabel(cmd)
	transactionHandlerDuration.WithLabelValues(cmdLabel).Observe(cost.Seconds())
	if code != g1_protocol.ErrorCode_ERR_OK && code != g1_protocol.ErrorCode_ERR_SUCESS {
		transactionHandlerErrors.WithLabelValues(cmdLabel, transactionCodeLabel(code)).Inc()
	}
}

func observeTransactionTimeout(stage string, cmd g1_protocol.CMD) {
	transactionTimeouts.WithLabelValues(stage, transactionCmdLabel(uint32(cmd))).Inc()
}

func sumTransactionMetric(read func(*TransactionMgr) float64) float64 {
	total := 0.0
	registeredTransactionMgrs.Range(func(key, _ any) bool {
		mgr, ok := key.(*TransactionMgr)
		if !ok || mgr == nil {
			return true
		}
		total += read(mgr)
		return true
	})
	return total
}

func sumTransactionQueueMetric(read func(*transactionShard) int) float64 {
	total := 0.0
	registeredTransactionMgrs.Range(func(key, _ any) bool {
		mgr, ok := key.(*TransactionMgr)
		if !ok || mgr == nil {
			return true
		}
		for _, shard := range mgr.shards {
			if shard == nil {
				continue
			}
			total += float64(read(shard))
		}
		return true
	})
	return total
}

func transactionCmdLabel(cmd uint32) string {
	//label := strings.TrimSpace(g1_protocol.CMD(cmd).String())
	label := g1_protocol.CMD(cmd).String()
	if label == "" || label == "0" {
		return "UNKNOWN"
	}
	return label
}

func transactionCodeLabel(code g1_protocol.ErrorCode) string {
	label := strings.TrimSpace(code.String())
	if label == "" || label == "0" {
		return g1_protocol.ErrorCode_ERR_INTERNAL.String()
	}
	return label
}
