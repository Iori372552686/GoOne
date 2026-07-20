package ssrpc

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type httpRouteKey struct {
	method string
	path   string
}

// grpcMethodEntry 描述注册到 Dispatcher 的单个 gRPC 方法。
type grpcMethodEntry struct {
	ServiceName    string // 如 "game.main.v1.MainService"
	MethodName     string // 如 "Login"
	IsServerStream bool

	// unary handler（!IsServerStream 时非 nil）。
	UnaryReqFactory func() any
	UnaryHandler    GRPCUnaryHandler
	// stream handler + descriptor（IsServerStream 时非 nil）。
	StreamHandler GRPCStreamHandler
	StreamDesc    *grpc.StreamDesc
}

// Dispatcher 是所有传输的统一注册中心。
//
// 它覆盖四条传输路径：
//   - cmd -> TransactionMgr handler（SSPacket）
//   - http(method+path) -> gin.HandlerFunc
//   - ws(cmd) -> CmdHandlerFunc（经 WebSocket 的 CSPacket）
//   - grpc(service/method) -> GRPCUnaryHandler / GRPCStreamHandler
//
// Dispatcher 有两个阶段：
//
//   - 可变（默认）：Register* 在 RWMutex 下向 handler map 追加。
//     DispatchWS/MountGin/MountGRPC 每次访问取读锁。
//   - Sealed（Seal 之后）：map 被冻结为只读副本，所有热路径查找变为无锁 map 读。
//     进一步的 Register* 调用被以 error 拒绝（Register*E）或记日志后忽略（遗留的
//     无返回值变体），使既有调用方继续可编译。
//
// Seal 正是使热路径免分配、免锁的关键（roadmap “Dispatcher 热路径无注册锁”）。一个
// Sealed 的 Dispatcher 无需任何同步即可安全并发 dispatch。
type Dispatcher struct {
	mu sync.RWMutex

	cmdHandlers  map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc
	httpHandlers map[httpRouteKey]gin.HandlerFunc
	wsHandlers   map[uint32]cmd_handler.CmdHandlerFunc
	grpcMethods  []grpcMethodEntry

	// sealed 以原子方式读取，使热路径连 RWMutex 都避开。
	sealed atomic.Bool
	// 由 Seal 填充的只读快照。Seal 前为 nil。
	cmdRO  map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc
	httpRO map[httpRouteKey]gin.HandlerFunc
	wsRO   map[uint32]cmd_handler.CmdHandlerFunc
	grpcRO []grpcMethodEntry
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		cmdHandlers:  make(map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc),
		httpHandlers: make(map[httpRouteKey]gin.HandlerFunc),
		wsHandlers:   make(map[uint32]cmd_handler.CmdHandlerFunc),
	}
}

// Sealed 上报是否已调用 Seal。一旦为 true，热路径 dispatch 方法执行无锁 map 读，
// 且不再允许注册。
func (d *Dispatcher) Sealed() bool {
	if d == nil {
		return false
	}
	return d.sealed.Load()
}

// Seal 冻结 Dispatcher：把当前 handler map 拷贝为只读快照，并把它翻入不可变阶段。
//
// Seal 幂等。Seal 之后，DispatchWS/MountGin/MountGRPC 执行无锁读，Register*E 返回
// ErrDispatcherSealed。遗留的无返回值 Register* 方法对 Seal 后的调用记日志后忽略，
// 使既有生成代码（调用 d.RegisterCmd）继续可编译、不变。
func (d *Dispatcher) Seal() {
	if d == nil || d.sealed.Load() {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sealed.Load() {
		return
	}
	d.cmdRO = make(map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc, len(d.cmdHandlers))
	for k, v := range d.cmdHandlers {
		d.cmdRO[k] = v
	}
	d.httpRO = make(map[httpRouteKey]gin.HandlerFunc, len(d.httpHandlers))
	for k, v := range d.httpHandlers {
		d.httpRO[k] = v
	}
	d.wsRO = make(map[uint32]cmd_handler.CmdHandlerFunc, len(d.wsHandlers))
	for k, v := range d.wsHandlers {
		d.wsRO[k] = v
	}
	d.grpcRO = append([]grpcMethodEntry(nil), d.grpcMethods...)
	d.sealed.Store(true)
}

// RegisterCmd 是遗留的无返回值 cmd 注册。保留它是为了生成代码与既有调用方；新代码
// 应优先用 RegisterCmdE（会暴露重复/sealed 错误）或 ssrpc.Registry。
//
// Seal 之后它是记日志的 no-op 而非 panic，使 Seal 后的生成注册不会令进程崩溃。
func (d *Dispatcher) RegisterCmd(cmd g1_protocol.CMD, h cmd_handler.CmdHandlerFunc) {
	if d == nil || h == nil {
		return
	}
	if d.sealed.Load() {
		logger.Errorf("ssrpc: RegisterCmd(%d) ignored: dispatcher already sealed", cmd)
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sealed.Load() {
		logger.Errorf("ssrpc: RegisterCmd(%d) ignored: dispatcher already sealed", cmd)
		return
	}
	d.cmdHandlers[cmd] = h
}

// RegisterCmdE 注册一个 cmd handler，并在 nil handler 或 dispatcher 已 sealed 时返回
// error。此处刻意不做重复 cmd 检测，以保留历史 last-write-wins 行为供直接调用方；
// 若需在装配期即时查重，请使用 ssrpc.Registry。
func (d *Dispatcher) RegisterCmdE(cmd g1_protocol.CMD, h cmd_handler.CmdHandlerFunc) error {
	if d == nil {
		return ErrNilDispatcher
	}
	if h == nil {
		return ErrNilHandler
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sealed.Load() {
		return ErrDispatcherSealed
	}
	d.cmdHandlers[cmd] = h
	return nil
}

func (d *Dispatcher) RegisterHTTP(method, path string, h gin.HandlerFunc) {
	if d == nil || h == nil {
		return
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = "POST"
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.httpHandlers[httpRouteKey{method: method, path: path}] = h
}

// MountGin 把所有已知 HTTP 路由挂载到给定 gin router/group。Seal 之后无锁遍历只读
// 快照。
func (d *Dispatcher) MountGin(r gin.IRoutes) {
	if d == nil || r == nil {
		return
	}
	if d.sealed.Load() {
		for k, h := range d.httpRO {
			r.Handle(k.method, k.path, h)
		}
		return
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for k, h := range d.httpHandlers {
		r.Handle(k.method, k.path, h)
	}
}

// RegisterToTransactionMgr registers all known cmd handlers into the TransactionMgr.
func (d *Dispatcher) RegisterToTransactionMgr(mgr transaction.ITransactionMgr) {
	if d == nil || mgr == nil {
		return
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	for cmd, h := range d.cmdHandlers {
		mgr.RegisterCmd(cmd, h)
	}
}

// ---------------------------------------------------------------------------
// WS (CSPacket) transport
// ---------------------------------------------------------------------------

// RegisterWS registers a WS (CSPacket) handler for a given cmd.
//
// The key is uint32 (matching CSPacketHeader.Cmd) to avoid a cast on every
// hot-path dispatch.
func (d *Dispatcher) RegisterWS(cmd uint32, h cmd_handler.CmdHandlerFunc) {
	if d == nil || h == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.wsHandlers[cmd] = h
}

// DispatchWS 按 cmd 查找 WS handler，若找到则执行它。
//
// 找到并执行 handler 时返回 (errorCode, true)。
// 当 cmd 没有注册 handler 时返回 (0, false)——调用方应回退到其默认转发逻辑（如
// router.SendMsgByConn）。
//
// Seal 之后这是热路径上的无锁 map 读。
func (d *Dispatcher) DispatchWS(ic cmd_handler.IContext, cmd uint32, body []byte) (g1_protocol.ErrorCode, bool) {
	if d == nil {
		return 0, false
	}
	if d.sealed.Load() {
		h, ok := d.wsRO[cmd]
		if !ok {
			return 0, false
		}
		return h(ic, body), true
	}
	d.mu.RLock()
	h, ok := d.wsHandlers[cmd]
	d.mu.RUnlock()
	if !ok {
		return 0, false
	}
	return h(ic, body), true
}

// ---------------------------------------------------------------------------
// gRPC transport
// ---------------------------------------------------------------------------

// RegisterGRPCUnary registers a gRPC unary handler for the given service/method.
func (d *Dispatcher) RegisterGRPCUnary(serviceName, methodName string, newReq func() any, h GRPCUnaryHandler) {
	if d == nil || newReq == nil || h == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.grpcMethods = append(d.grpcMethods, grpcMethodEntry{
		ServiceName:    serviceName,
		MethodName:     methodName,
		UnaryReqFactory: newReq,
		UnaryHandler:   h,
	})
}

// RegisterGRPCStream registers a gRPC server-streaming handler.
func (d *Dispatcher) RegisterGRPCStream(serviceName, methodName string, h GRPCStreamHandler) {
	if d == nil || h == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.grpcMethods = append(d.grpcMethods, grpcMethodEntry{
		ServiceName:    serviceName,
		MethodName:     methodName,
		IsServerStream: true,
		StreamHandler:  h,
		StreamDesc: &grpc.StreamDesc{
			StreamName:    methodName,
			ServerStreams:  true,
			ClientStreams:  false,
		},
	})
}

// MountGRPC 把所有已收集的 gRPC 方法挂载到给定 grpc.Server。
//
// 它按 service name 分组并为每个调用 srv.RegisterService。Seal 之后读取冻结快照；
// gRPC service 分组已预算，使注册不产生每调用分配。
func (d *Dispatcher) MountGRPC(srv *grpc.Server) {
	if d == nil || srv == nil {
		return
	}
	methods := d.grpcMethods
	if d.sealed.Load() {
		methods = d.grpcRO
	} else {
		d.mu.RLock()
		methods = append([]grpcMethodEntry(nil), d.grpcMethods...)
		d.mu.RUnlock()
	}

	type svcBucket struct {
		methods []grpc.MethodDesc
		streams []grpc.StreamDesc
	}
	grouped := make(map[string]*svcBucket)

	for i := range methods {
		m := &methods[i]
		bucket, ok := grouped[m.ServiceName]
		if !ok {
			bucket = &svcBucket{}
			grouped[m.ServiceName] = bucket
		}
		if m.IsServerStream {
			sd := *m.StreamDesc
			handler := m.StreamHandler // capture for closure
			sd.Handler = func(srv any, stream grpc.ServerStream) error {
				return handler(srv, stream)
			}
			bucket.streams = append(bucket.streams, sd)
		} else {
			reqFactory := m.UnaryReqFactory
			handler := m.UnaryHandler // capture for closure
			bucket.methods = append(bucket.methods, grpc.MethodDesc{
				MethodName: m.MethodName,
				Handler:    makeGRPCMethodHandler(m.ServiceName, m.MethodName, reqFactory, handler),
			})
		}
	}

	for svcName, bucket := range grouped {
		desc := &grpc.ServiceDesc{
			ServiceName: svcName,
			HandlerType: (*any)(nil),
			Methods:     bucket.methods,
			Streams:     bucket.streams,
			Metadata:    svcName,
		}
		srv.RegisterService(desc, nil)
	}
}

// makeGRPCMethodHandler adapts a GRPCUnaryHandler to the grpc.MethodDesc.Handler
// signature. The dec callback receives a pointer to any; gRPC populates it with
// the decoded proto message.
func makeGRPCMethodHandler(serviceName, methodName string, newReq func() any, h GRPCUnaryHandler) func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	return func(_ any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		reqAny := newReq()
		req, ok := reqAny.(proto.Message)
		if !ok || req == nil {
			return nil, ToGRPCError(g1_protocol.ErrorCode_ERR_INTERNAL)
		}
		if err := dec(req); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return h(ctx, req)
		}
		info := &grpc.UnaryServerInfo{
			FullMethod: "/" + serviceName + "/" + methodName,
		}
		return interceptor(ctx, req, info, func(ctx context.Context, r any) (any, error) {
			return h(ctx, r)
		})
	}
}

