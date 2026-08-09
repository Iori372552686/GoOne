package ssrpc

import (
	"context"
	"net/http"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

type Transport string

const (
	TransportSS   Transport = "sspack"
	TransportHTTP Transport = "http"
	TransportTCP  Transport = "tcp"
	TransportWS   Transport = "ws"
	TransportGRPC Transport = "grpc"
)

// Context is the unified request context for GoOne RPC handlers (Phase A).
//
// It wraps the existing cmd_handler.IContext (implemented by Transaction),
// and adds metadata that is useful for middleware and logging.
type Context struct {
	context.Context
	cmd_handler.IContext

	cancel context.CancelFunc

	Transport Transport
	Cmd       g1_protocol.CMD
	Session   Session
	MCP       MCP // optional capability provider (Phase A+)

	// Method is the logical RPC method name (typically "Service.Method" or comment).
	Method string

	// HTTP transports attach raw request metadata here for middleware such as
	// signature verification. Non-HTTP transports leave these fields empty.
	HTTPRequest *http.Request
	HTTPBody    []byte

	// Flags propagated from ssrpc.MethodDesc (set by WrapUnary).
	AuthRequired bool
	SignRequired bool

	// TraceTags are optional extra tags for tracing/metrics.
	TraceTags map[string]string

	// UIDLocker can be attached via middleware; UIDLock() will prefer it when present.
	UIDLocker UIDLocker
}

func WrapIContext(ic cmd_handler.IContext, cmd g1_protocol.CMD) *Context {
	return WrapIContextWithContext(baseContextFromIContext(ic), ic, cmd)
}

func WrapIContextWithContext(base context.Context, ic cmd_handler.IContext, cmd g1_protocol.CMD) *Context {
	if base == nil {
		base = context.Background()
	}

	session := buildSession(ic, TransportSS, cmd)
	return &Context{
		Context:   base,
		IContext:  ic,
		Transport: session.Transport,
		Cmd:       session.Cmd,
		Session:   session,
	}
}

func (c *Context) SetTransport(t Transport) {
	if c == nil {
		return
	}
	c.Transport = t
	c.Session.Transport = t
}

func (c *Context) SetCmd(cmd g1_protocol.CMD) {
	if c == nil {
		return
	}
	c.Cmd = cmd
	c.Session.Cmd = cmd
}

func (c *Context) SetMethod(name string) {
	if c == nil {
		return
	}
	c.Method = name
	c.Session.Method = name
}

func (c *Context) SetHTTPRequest(req *http.Request, body []byte) {
	if c == nil {
		return
	}
	c.HTTPRequest = req
	if len(body) == 0 {
		c.HTTPBody = nil
		return
	}
	c.HTTPBody = append([]byte(nil), body...)
}

func (c *Context) ApplyTimeout(timeout time.Duration) context.CancelFunc {
	if c == nil || timeout <= 0 {
		return func() {}
	}

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}

	base := c.Context
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, timeout)
	c.Context = ctx
	c.cancel = cancel
	return cancel
}

func (c *Context) Close() {
	if c == nil || c.cancel == nil {
		return
	}
	c.cancel()
	c.cancel = nil
}

func (c *Context) TraceID() string {
	if c == nil {
		return ""
	}
	return traceIDFromContext(c.Context)
}

func (c *Context) SpanID() string {
	if c == nil {
		return ""
	}
	return spanIDFromContext(c.Context)
}

func (c *Context) traceLogPrefix() string {
	traceID := c.TraceID()
	if traceID == "" {
		return ""
	}
	spanID := c.SpanID()
	if spanID == "" {
		return "[trace_id:" + traceID + "] "
	}
	return "[trace_id:" + traceID + " span_id:" + spanID + "] "
}

func (c *Context) Errorf(format string, args ...interface{}) {
	if c != nil && c.IContext != nil {
		c.IContext.Errorf(c.traceLogPrefix()+format, args...)
		return
	}
	logger.Errorf(c.traceLogPrefix()+format, args...)
}

func (c *Context) Warningf(format string, args ...interface{}) {
	if c != nil && c.IContext != nil {
		c.IContext.Warningf(c.traceLogPrefix()+format, args...)
		return
	}
	logger.Warningf(c.traceLogPrefix()+format, args...)
}

func (c *Context) Infof(format string, args ...interface{}) {
	if c != nil && c.IContext != nil {
		c.IContext.Infof(c.traceLogPrefix()+format, args...)
		return
	}
	logger.Infof(c.traceLogPrefix()+format, args...)
}

func (c *Context) Debugf(format string, args ...interface{}) {
	if c != nil && c.IContext != nil {
		c.IContext.Debugf(c.traceLogPrefix()+format, args...)
		return
	}
	logger.Debugf(c.traceLogPrefix()+format, args...)
}

func baseContextFromIContext(ic cmd_handler.IContext) context.Context {
	if v, ok := any(ic).(interface{ Context() context.Context }); ok {
		if ctx := v.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func buildSession(ic cmd_handler.IContext, transport Transport, cmd g1_protocol.CMD) Session {
	s := Session{
		Transport: transport,
		Cmd:       cmd,
	}
	if ic == nil {
		return s
	}

	s.UID = ic.Uid()
	s.Zone = ic.Zone()
	s.RID = ic.Rid()
	s.SrcBusID = ic.OriSrcBusId()
	s.PeerIP = ic.Ip()
	s.PeerFlag = ic.Flag()

	if v, ok := any(ic).(interface{ TransID() uint32 }); ok {
		s.TransID = v.TransID()
	}

	return s
}
