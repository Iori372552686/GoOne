package ssrpc

import (
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestDefaultMiddlewares_UseBuiltInPrometheusMetrics(t *testing.T) {
	const method = "test.default.metrics.timeout"
	cmd := g1_protocol.CMD(0x7F0001)
	ctx := WrapIContext(&fakeIContext{uid: 42}, cmd)
	ctx.SetTransport(TransportSS)
	ctx.SetMethod(method)

	mws := DefaultMiddlewares(DefaultMWOptions{})
	h := Chain(mws...)(func(ctx *Context, req proto.Message) (proto.Message, error) {
		return nil, E(g1_protocol.ErrorCode_ERR_TIMEOUT, "timeout")
	})

	reqTotalBefore := metricValue(t, ssrpcRequestTotal.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_TIMEOUT.String(),
	))
	errTotalBefore := metricValue(t, ssrpcRequestErrors.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_TIMEOUT.String(),
	))
	timeoutBefore := metricValue(t, ssrpcRequestTimeouts.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
	))

	_, err := h(ctx, &fakePB{})
	if ToErrorCode(err) != g1_protocol.ErrorCode_ERR_TIMEOUT {
		t.Fatalf("expected ERR_TIMEOUT, got code=%v err=%v", ToErrorCode(err), err)
	}

	if got := metricValue(t, ssrpcRequestTotal.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_TIMEOUT.String(),
	)); got != reqTotalBefore+1 {
		t.Fatalf("requests_total delta=%v, want 1", got-reqTotalBefore)
	}
	if got := metricValue(t, ssrpcRequestErrors.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_TIMEOUT.String(),
	)); got != errTotalBefore+1 {
		t.Fatalf("errors_total delta=%v, want 1", got-errTotalBefore)
	}
	if got := metricValue(t, ssrpcRequestTimeouts.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
	)); got != timeoutBefore+1 {
		t.Fatalf("timeouts_total delta=%v, want 1", got-timeoutBefore)
	}
	if got := metricValue(t, ssrpcRequestsInFlight.WithLabelValues(
		string(TransportSS),
		method,
		ssrpcCmdLabel(cmd),
	)); got != 0 {
		t.Fatalf("requests_in_flight=%v, want 0", got)
	}
}

func TestDefaultMetricsRecorder_RecordsSuccess(t *testing.T) {
	const method = "test.default.metrics.ok"
	cmd := g1_protocol.CMD(0x7F0002)
	ctx := WrapIContext(&fakeIContext{uid: 7}, cmd)
	ctx.SetTransport(TransportHTTP)
	ctx.SetMethod(method)

	h := Metrics(DefaultMetricsRecorder())(func(ctx *Context, req proto.Message) (proto.Message, error) {
		return &fakePB{}, nil
	})

	reqTotalBefore := metricValue(t, ssrpcRequestTotal.WithLabelValues(
		string(TransportHTTP),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_OK.String(),
	))

	_, err := h(ctx, &fakePB{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	if got := metricValue(t, ssrpcRequestTotal.WithLabelValues(
		string(TransportHTTP),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_OK.String(),
	)); got != reqTotalBefore+1 {
		t.Fatalf("requests_total delta=%v, want 1", got-reqTotalBefore)
	}
	if got := metricValue(t, ssrpcRequestErrors.WithLabelValues(
		string(TransportHTTP),
		method,
		ssrpcCmdLabel(cmd),
		g1_protocol.ErrorCode_ERR_OK.String(),
	)); got != 0 {
		t.Fatalf("errors_total=%v, want 0", got)
	}
}

func metricValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()
	dtoMetric := &dto.Metric{}
	if err := metric.Write(dtoMetric); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	if counter := dtoMetric.GetCounter(); counter != nil {
		return counter.GetValue()
	}
	if gauge := dtoMetric.GetGauge(); gauge != nil {
		return gauge.GetValue()
	}
	t.Fatalf("unsupported metric type: %#v", dtoMetric)
	return 0
}
