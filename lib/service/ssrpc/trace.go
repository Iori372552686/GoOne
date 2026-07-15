package ssrpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type traceContextKey string

const (
	traceIDContextKey      traceContextKey = "goone.ssrpc.trace_id"
	spanIDContextKey       traceContextKey = "goone.ssrpc.span_id"
	parentSpanIDContextKey traceContextKey = "goone.ssrpc.parent_span_id"
	traceparentHeader                      = "traceparent"
	traceIDHeader                          = "x-trace-id"
	defaultTracingExporter                 = "stdout"
)

var (
	globalTraceProviderMu sync.RWMutex
	globalTraceProvider   TraceProvider
	// otelShutdown 存储 OTel SDK TracerProvider 的 shutdown 函数（otlphttp 模式）。
	otelShutdown func(context.Context) error
)

// TraceProvider is a pluggable tracing hook.
//
// Start should return a finish callback (may be nil). finish is always called with the handler error.
type TraceProvider interface {
	Start(ctx *Context, tags map[string]string) (finish func(err error))
}

type TracingConfig struct {
	Enabled      bool              `json:"enabled" yaml:"enabled"`
	Exporter     string            `json:"exporter" yaml:"exporter"`
	Endpoint     string            `json:"endpoint" yaml:"endpoint"`
	Insecure     bool              `json:"insecure" yaml:"insecure"`
	SamplerRatio float64           `json:"sampler_ratio" yaml:"sampler_ratio"`
	Headers      map[string]string `json:"headers" yaml:"headers"`
}

type minimalTraceProvider struct {
	serviceName  string
	exporter     string
	samplerRatio float64
}

func SetGlobalTraceProvider(tp TraceProvider) {
	globalTraceProviderMu.Lock()
	defer globalTraceProviderMu.Unlock()
	globalTraceProvider = tp
}

func GlobalTraceProvider() TraceProvider {
	globalTraceProviderMu.RLock()
	defer globalTraceProviderMu.RUnlock()
	return globalTraceProvider
}

func InitTracing(serviceName string, cfg TracingConfig) error {
	if !cfg.Enabled {
		SetGlobalTraceProvider(nil)
		return nil
	}
	exporter := strings.TrimSpace(strings.ToLower(cfg.Exporter))
	if exporter == "" {
		exporter = defaultTracingExporter
	}
	switch exporter {
	case "stdout", "otlphttp":
	default:
		return fmt.Errorf("unsupported tracing exporter %q", cfg.Exporter)
	}
	if cfg.SamplerRatio < 0 || cfg.SamplerRatio > 1 {
		return fmt.Errorf("invalid tracing sampler_ratio %v", cfg.SamplerRatio)
	}
	if cfg.SamplerRatio == 0 {
		cfg.SamplerRatio = 1
	}

	if exporter == "otlphttp" {
		tp, err := newOtelTraceProvider(serviceName, cfg)
		if err != nil {
			return fmt.Errorf("init otlphttp trace exporter: %w", err)
		}
		SetGlobalTraceProvider(tp)
		return nil
	}

	// stdout 模式：保持原有轻量 provider
	SetGlobalTraceProvider(&minimalTraceProvider{
		serviceName:  serviceName,
		exporter:     exporter,
		samplerRatio: cfg.SamplerRatio,
	})
	return nil
}

func ShutdownTracing(ctx context.Context) error {
	SetGlobalTraceProvider(nil)
	globalTraceProviderMu.Lock()
	shutdown := otelShutdown
	otelShutdown = nil
	globalTraceProviderMu.Unlock()
	if shutdown != nil {
		return shutdown(ctx)
	}
	return nil
}

func resolveTraceProvider(tp TraceProvider) TraceProvider {
	if tp != nil {
		return tp
	}
	return GlobalTraceProvider()
}

func buildTraceTags(ctx *Context) map[string]string {
	tags := map[string]string{
		"cmd":       strconv.FormatUint(uint64(ctx.Cmd), 10),
		"method":    ctx.Method,
		"transport": string(ctx.Session.Transport),
	}
	if ctx.Session.UID != 0 {
		tags["uid"] = strconv.FormatUint(ctx.Session.UID, 10)
	}
	if ctx.Session.Zone != 0 {
		tags["zone"] = strconv.FormatUint(uint64(ctx.Session.Zone), 10)
	}
	if ctx.Session.RID != 0 {
		tags["rid"] = strconv.FormatUint(ctx.Session.RID, 10)
	}
	if ctx.Session.TransID != 0 {
		tags["trans_id"] = strconv.FormatUint(uint64(ctx.Session.TransID), 10)
	}
	if ctx.TraceTags != nil {
		for k, v := range ctx.TraceTags {
			tags[k] = v
		}
	}
	return tags
}

func mergeTraceTags(base map[string]string, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	merged := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return merged
}

func StartTrace(ctx *Context, tp TraceProvider, extraTags map[string]string) func(err error) {
	if ctx == nil {
		return nil
	}
	resolved := resolveTraceProvider(tp)
	if resolved == nil {
		return nil
	}
	return resolved.Start(ctx, mergeTraceTags(buildTraceTags(ctx), extraTags))
}

func (p *minimalTraceProvider) Start(ctx *Context, tags map[string]string) func(err error) {
	if p == nil || ctx == nil {
		return nil
	}
	base := ctx.Context
	if base == nil {
		base = context.Background()
	}
	traceID := traceIDFromContext(base)
	if traceID == "" {
		traceID = randomHex(16)
	}
	if !shouldSample(traceID, p.samplerRatio) {
		return nil
	}
	parentSpanID := spanIDFromContext(base)
	spanID := randomHex(8)
	ctx.Context = contextWithTraceValues(base, traceID, spanID, parentSpanID)

	spanName := strings.TrimSpace(tags["span.name"])
	if spanName == "" {
		spanName = strings.TrimSpace(ctx.Method)
	}
	if spanName == "" {
		spanName = fmt.Sprintf("ssrpc.%s.%d", ctx.Transport, uint32(ctx.Cmd))
	}
	spanKind := strings.TrimSpace(tags["span.kind"])
	if spanKind == "" {
		spanKind = "internal"
	}
	startedAt := time.Now()
	return func(err error) {
		if p.exporter != "stdout" {
			return
		}
		cost := time.Since(startedAt)
		if err != nil {
			ctx.Warningf("trace err {service:%s, name:%s, kind:%s, parent_span_id:%s, cost:%v} err=%v",
				p.serviceName, spanName, spanKind, parentSpanID, cost, err)
			return
		}
		ctx.Debugf("trace ok {service:%s, name:%s, kind:%s, parent_span_id:%s, cost:%v}",
			p.serviceName, spanName, spanKind, parentSpanID, cost)
	}
}

func contextWithTraceValues(base context.Context, traceID, spanID, parentSpanID string) context.Context {
	if base == nil {
		base = context.Background()
	}
	if traceID != "" {
		base = context.WithValue(base, traceIDContextKey, traceID)
	}
	if spanID != "" {
		base = context.WithValue(base, spanIDContextKey, spanID)
	}
	if parentSpanID != "" {
		base = context.WithValue(base, parentSpanIDContextKey, parentSpanID)
	}
	return base
}

func traceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, _ := ctx.Value(traceIDContextKey).(string); isValidHex(traceID, 32) {
		return strings.ToLower(traceID)
	}
	if traceID, _ := ctx.Value(string(traceIDContextKey)).(string); isValidHex(traceID, 32) {
		return strings.ToLower(traceID)
	}
	return ""
}

func spanIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if spanID, _ := ctx.Value(spanIDContextKey).(string); isValidHex(spanID, 16) {
		return strings.ToLower(spanID)
	}
	if spanID, _ := ctx.Value(string(spanIDContextKey)).(string); isValidHex(spanID, 16) {
		return strings.ToLower(spanID)
	}
	return ""
}

func ExtractHTTPTraceContext(req *http.Request) context.Context {
	if req == nil {
		return context.Background()
	}
	base := req.Context()
	traceID, spanID := extractTraceHeaders(req.Header.Get(traceparentHeader), req.Header.Get(traceIDHeader))
	return contextWithTraceValues(base, traceID, spanID, "")
}

func WriteHTTPTraceResponse(ctx *Context, headers http.Header) {
	if ctx == nil || headers == nil {
		return
	}
	if traceID := ctx.TraceID(); traceID != "" {
		headers.Set("X-Trace-Id", traceID)
	}
	if traceparent := buildTraceparent(ctx.TraceID(), ctx.SpanID()); traceparent != "" {
		headers.Set("Traceparent", traceparent)
	}
}

func ExtractGRPCTraceContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	traceID, spanID := extractTraceHeaders(firstMetadata(md, traceparentHeader), firstMetadata(md, traceIDHeader))
	return contextWithTraceValues(ctx, traceID, spanID, "")
}

func WriteGRPCTraceResponse(baseCtx context.Context, traceCtx *Context) {
	if baseCtx == nil || traceCtx == nil {
		return
	}
	md := metadata.New(nil)
	if traceID := traceCtx.TraceID(); traceID != "" {
		md.Set(traceIDHeader, traceID)
	}
	if traceparent := buildTraceparent(traceCtx.TraceID(), traceCtx.SpanID()); traceparent != "" {
		md.Set(traceparentHeader, traceparent)
	}
	_ = grpc.SetHeader(baseCtx, md)
	_ = grpc.SetTrailer(baseCtx, md)
}

func WriteGRPCStreamTraceResponse(stream grpc.ServerStream, traceCtx *Context) {
	if stream == nil || traceCtx == nil {
		return
	}
	md := metadata.New(nil)
	if traceID := traceCtx.TraceID(); traceID != "" {
		md.Set(traceIDHeader, traceID)
	}
	if traceparent := buildTraceparent(traceCtx.TraceID(), traceCtx.SpanID()); traceparent != "" {
		md.Set(traceparentHeader, traceparent)
	}
	_ = stream.SetHeader(md)
	stream.SetTrailer(md)
}

func extractTraceHeaders(traceparent, traceID string) (string, string) {
	if tID, sID, ok := parseTraceparent(traceparent); ok {
		return tID, sID
	}
	traceID = strings.TrimSpace(strings.ToLower(traceID))
	if isValidHex(traceID, 32) {
		return traceID, ""
	}
	return "", ""
}

func parseTraceparent(v string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(strings.ToLower(v)), "-")
	if len(parts) != 4 {
		return "", "", false
	}
	if !isValidHex(parts[1], 32) || !isValidHex(parts[2], 16) {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func buildTraceparent(traceID, spanID string) string {
	if !isValidHex(traceID, 32) || !isValidHex(spanID, 16) {
		return ""
	}
	return "00-" + traceID + "-" + spanID + "-01"
}

func randomHex(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n*2)
	}
	return hex.EncodeToString(buf)
}

func isValidHex(v string, wantLen int) bool {
	if len(v) != wantLen {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

func shouldSample(traceID string, ratio float64) bool {
	if ratio >= 1 {
		return true
	}
	if ratio <= 0 {
		return false
	}
	if len(traceID) < 8 {
		return true
	}
	prefix, err := strconv.ParseUint(traceID[:8], 16, 32)
	if err != nil {
		return true
	}
	return float64(prefix)/float64(^uint32(0)) <= ratio
}

func firstMetadata(md metadata.MD, key string) string {
	if md == nil {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// TraceWith runs tracing via the provided TraceProvider (falling back to the global provider).
func TraceWith(tp TraceProvider) Middleware {
	return func(next Handler) Handler {
		return func(ctx *Context, req proto.Message) (proto.Message, error) {
			if ctx == nil {
				return next(ctx, req)
			}
			finish := StartTrace(ctx, tp, map[string]string{
				"span.name":  ctx.Method,
				"span.kind":  "internal",
				"span.phase": "handler",
			})
			rsp, err := next(ctx, req)
			if finish != nil {
				finish(err)
			}
			return rsp, err
		}
	}
}

// Trace keeps backward-compatibility and stays a no-op by default.
func Trace() Middleware {
	return TraceWith(nil)
}

// ---------------- OTLP HTTP exporter (otlphttp) ----------------

// newOtelTraceProvider 创建基于 OTel SDK 的 TracerProvider，通过 OTLP HTTP 导出 span 到 collector。
// endpoint 形如 "localhost:4318"（不含 scheme）；insecure=true 用 HTTP，否则 HTTPS。
func newOtelTraceProvider(serviceName string, cfg TracingConfig) (TraceProvider, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("tracing.endpoint is required for otlphttp exporter")
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}

	exp, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, err
	}

	ratio := cfg.SamplerRatio
	if ratio <= 0 {
		ratio = 1
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ratio)),
	)
	otel.SetTracerProvider(tp)

	globalTraceProviderMu.Lock()
	otelShutdown = tp.Shutdown
	globalTraceProviderMu.Unlock()

	return &otelTraceProvider{tracer: tp.Tracer("goone.ssrpc")}, nil
}

// otelTraceProvider 用 OTel SDK API 创建 span，经 BatchSpanProcessor 异步导出到 collector。
type otelTraceProvider struct {
	tracer oteltrace.Tracer
}

func (p *otelTraceProvider) Start(ctx *Context, tags map[string]string) func(err error) {
	if p == nil || ctx == nil {
		return nil
	}
	base := ctx.Context
	if base == nil {
		base = context.Background()
	}

	spanName := strings.TrimSpace(tags["span.name"])
	if spanName == "" {
		spanName = strings.TrimSpace(ctx.Method)
	}
	if spanName == "" {
		spanName = fmt.Sprintf("ssrpc.%s.%d", ctx.Transport, uint32(ctx.Cmd))
	}

	attrs := make([]attribute.KeyValue, 0, len(tags))
	for k, v := range tags {
		if k == "span.name" || k == "span.kind" {
			continue
		}
		attrs = append(attrs, attribute.String(k, v))
	}

	otelCtx, span := p.tracer.Start(base, spanName, oteltrace.WithAttributes(attrs...))
	ctx.Context = otelCtx

	return func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()
	}
}
