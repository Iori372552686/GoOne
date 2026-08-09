package ssrpc

import (
	"errors"
	"strings"
	"sync"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// BindingKind 按 Binding 所目标的传输分类。每种 kind 有独立唯一 key 空间，与
// 按传输唯一性规则一致。
type BindingKind uint8

const (
	// BindingInvalid 是零值，会被 Register 拒绝。
	BindingInvalid BindingKind = iota
	// BindingCMD 目标 SSPacket cmd 路径（key：g1_protocol.CMD）。
	BindingCMD
	// BindingHTTP 目标 gin HTTP 路径（key：大写 METHOD + path）。
	BindingHTTP
	// BindingWS 目标经 WebSocket 的 CSPacket 路径（key：uint32 cmd）。
	BindingWS
	// BindingGRPCUnary 目标 gRPC unary 方法（key：service + "/" + method）。
	BindingGRPCUnary
	// BindingGRPCStream 目标 gRPC server-streaming 方法。
	BindingGRPCStream
)

// Binding 声明一条传输 handler 注册项。按 Kind 恰好设置一个 handler 字段：
//
//   - BindingCMD / BindingWS：CmdHandler（一个 cmd_handler.CmdHandlerFunc，即
//     WrapUnary / WrapWS 的输出）。
//   - BindingHTTP：HTTPHandler（一个 gin.HandlerFunc，即 WrapHTTPGin 的输出）。
//   - BindingGRPCUnary：UnaryHandler + ReqFactory。
//   - BindingGRPCStream：StreamHandler。
//
// Binding 携带的是已包装的 handler（中间件已应用），故 Registry 不依赖
// MethodDesc/Wrap* 细节。
type Binding struct {
	Kind    BindingKind
	Service string

	// CMD 是 BindingCMD 与 BindingWS 的唯一 key（WS 用 uint32(CMD)）。
	CMD g1_protocol.CMD

	// HTTP method + path，Kind == BindingHTTP 时使用。Method 会被规范化（大写、默认
	// POST）；path 会被 trim。
	HTTPMethod string
	HTTPPath   string

	// GRPCService / GRPCMethod 在 Kind 为 gRPC kind 时标识一个 gRPC 方法。
	GRPCService string
	GRPCMethod  string

	// handlers。恰好匹配 Kind 的那个必须非 nil。
	CmdHandler    cmd_handler.CmdHandlerFunc
	HTTPHandler   gin.HandlerFunc
	UnaryHandler  GRPCUnaryHandler
	ReqFactory    func() any
	StreamHandler GRPCStreamHandler
}

// key 返回本 binding 在其 kind 内的唯一 key 字符串，并上报 binding 是否良构。kind
// 被折入 key，故两种不同 kind 的 binding 即便字符串形式恰好相同也绝不冲突。
func (b Binding) key() (string, bool) {
	switch b.Kind {
	case BindingCMD:
		return "cmd:" + itoa(int(b.CMD)), true
	case BindingWS:
		return "ws:" + itoa(int(b.CMD)), true
	case BindingHTTP:
		method := strings.ToUpper(strings.TrimSpace(b.HTTPMethod))
		if method == "" {
			method = "POST"
		}
		path := strings.TrimSpace(b.HTTPPath)
		if path == "" {
			return "", false
		}
		return "http:" + method + ":" + path, true
	case BindingGRPCUnary, BindingGRPCStream:
		svc := strings.TrimSpace(b.GRPCService)
		mtd := strings.TrimSpace(b.GRPCMethod)
		if svc == "" || mtd == "" {
			return "", false
		}
		prefix := "grpc-u:"
		if b.Kind == BindingGRPCStream {
			prefix = "grpc-s:"
		}
		return prefix + svc + "/" + mtd, true
	default:
		return "", false
	}
}

// validate 检查匹配 Kind 的 handler 字段是否已设置。
func (b Binding) validate() error {
	switch b.Kind {
	case BindingCMD, BindingWS:
		if b.CmdHandler == nil {
			return ErrNilHandler
		}
	case BindingHTTP:
		if b.HTTPHandler == nil {
			return ErrNilHandler
		}
	case BindingGRPCUnary:
		if b.UnaryHandler == nil || b.ReqFactory == nil {
			return ErrNilHandler
		}
	case BindingGRPCStream:
		if b.StreamHandler == nil {
			return ErrNilHandler
		}
	default:
		return ErrInvalidBinding
	}
	return nil
}

// itoa 是用于 key 的小型免分配 int->string；它避免把 strconv 拉入热路径，并保持
// key 确定性。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Registry 是 SSRPC binding 的装配期容器。Module 以批次注册其服务的 binding；
// Register 原子地校验整批（批次内与跨批次查重），仅当每条 binding 良构时才提交。
// Seal 随后把 Registry 冻结为一个不可变、已 sealed 的 Dispatcher。
//
// Registry 是 app 实例级的（绝不包级全局），且不使用 func init 注册。
type Registry struct {
	mu     sync.Mutex
	sealed bool

	// 已收集的 binding，按唯一 key 字符串索引。所有 kind 共用同一 map，因为 kind
	// 已折入 key。
	keys      map[string]Binding
	cmdOrder  []g1_protocol.CMD
	httpOrder []httpRouteKey
	wsOrder   []uint32
	grpcOrder []grpcMethodEntry

	// sealedDispatcher 缓存首次 Seal 的结果，使 Seal 真正幂等（后续调用返回
	// 同一个 Dispatcher）。历史实现二次 Seal 返回 ErrRegistrySealed，与文档承诺矛盾。
	sealedDispatcher *Dispatcher
}

// NewRegistry 构建一个空的、未 Seal 的 Registry。
func NewRegistry() *Registry {
	return &Registry{keys: make(map[string]Binding)}
}

// Register 校验并提交一个服务的整批 binding。批次是原子的：若任何 binding 非法，
// 或任何 key 与同批另一 binding 或已注册状态冲突，整批被拒绝且 Registry 保持不
// 变。返回的 error 合并所有检测到的问题，使调用方一次看到全部问题。
//
// service 必须非空，是逻辑服务名（用于错误消息与未来指标）。
//
// nil 接收者返回 ErrNilRegistry（历史返回 ErrNilDispatcher，语义不准）。
func (r *Registry) Register(service string, bindings ...Binding) error {
	if r == nil {
		return ErrNilRegistry
	}
	if strings.TrimSpace(service) == "" {
		return ErrEmptyService
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		return ErrRegistrySealed
	}

	// 先校验并检测批次内重复，不改动状态。
	seen := make(map[string]struct{}, len(bindings))
	var errs []error
	for i, b := range bindings {
		b.Service = service
		if err := b.validate(); err != nil {
			errs = append(errs, wrapBindingErr(service, i, err))
			continue
		}
		k, ok := b.key()
		if !ok {
			errs = append(errs, wrapBindingErr(service, i, ErrInvalidBinding))
			continue
		}
		if _, dup := seen[k]; dup {
			errs = append(errs, wrapBindingErr(service, i, ErrDuplicateBinding))
			continue
		}
		if _, dup := r.keys[k]; dup {
			errs = append(errs, wrapBindingErr(service, i, ErrDuplicateBinding))
			continue
		}
		seen[k] = struct{}{}
	}
	if len(errs) > 0 {
		// 不部分提交：保持 r.keys 不变。
		return errors.Join(errs...)
	}

	// 提交。
	for _, b := range bindings {
		b.Service = service
		k, _ := b.key()
		r.keys[k] = b
		switch b.Kind {
		case BindingCMD:
			r.cmdOrder = append(r.cmdOrder, b.CMD)
		case BindingWS:
			r.wsOrder = append(r.wsOrder, uint32(b.CMD))
		case BindingHTTP:
			method := strings.ToUpper(strings.TrimSpace(b.HTTPMethod))
			if method == "" {
				method = "POST"
			}
			r.httpOrder = append(r.httpOrder, httpRouteKey{method: method, path: strings.TrimSpace(b.HTTPPath)})
		case BindingGRPCUnary, BindingGRPCStream:
			entry := grpcMethodEntry{
				ServiceName:     b.GRPCService,
				MethodName:      b.GRPCMethod,
				IsServerStream:  b.Kind == BindingGRPCStream,
				UnaryReqFactory: b.ReqFactory,
				UnaryHandler:    b.UnaryHandler,
				StreamHandler:   b.StreamHandler,
			}
			if b.Kind == BindingGRPCStream {
				entry.StreamDesc = &grpc.StreamDesc{
					StreamName:    b.GRPCMethod,
					ServerStreams: true,
					ClientStreams: false,
				}
			}
			r.grpcOrder = append(r.grpcOrder, entry)
		}
	}
	return nil
}

// Seal 冻结 Registry，并从已注册 binding 构建一个不可变 Dispatcher。返回的
// Dispatcher 已 Seal，故其热路径无锁。
//
// Seal 真正幂等——缓存并重复返回同一个 Dispatcher（历史二次调用返回
// ErrRegistrySealed，与文档承诺矛盾）。Seal 后再 Register 仍返回 ErrRegistrySealed。
func (r *Registry) Seal() (*Dispatcher, error) {
	if r == nil {
		return nil, ErrNilRegistry
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sealed {
		// 幂等：返回缓存的 Dispatcher（若有），否则（异常路径）返回 ErrRegistrySealed。
		if r.sealedDispatcher != nil {
			return r.sealedDispatcher, nil
		}
		return nil, ErrRegistrySealed
	}
	r.sealed = true

	d := &Dispatcher{
		cmdHandlers:  make(map[g1_protocol.CMD]cmd_handler.CmdHandlerFunc, len(r.cmdOrder)),
		httpHandlers: make(map[httpRouteKey]gin.HandlerFunc, len(r.httpOrder)),
		wsHandlers:   make(map[uint32]cmd_handler.CmdHandlerFunc, len(r.wsOrder)),
		grpcMethods:  make([]grpcMethodEntry, len(r.grpcOrder)),
	}
	for _, cmd := range r.cmdOrder {
		k, _ := (Binding{Kind: BindingCMD, CMD: cmd}).key()
		if b, ok := r.keys[k]; ok {
			d.cmdHandlers[cmd] = b.CmdHandler
		}
	}
	for _, ws := range r.wsOrder {
		k, _ := (Binding{Kind: BindingWS, CMD: g1_protocol.CMD(ws)}).key()
		if b, ok := r.keys[k]; ok {
			d.wsHandlers[ws] = b.CmdHandler
		}
	}
	for _, hk := range r.httpOrder {
		k, _ := (Binding{Kind: BindingHTTP, HTTPMethod: hk.method, HTTPPath: hk.path}).key()
		if b, ok := r.keys[k]; ok {
			d.httpHandlers[hk] = b.HTTPHandler
		}
	}
	copy(d.grpcMethods, r.grpcOrder)
	d.Seal()
	r.sealedDispatcher = d
	return d, nil
}

// IsSealed 上报是否已调用 Seal。
func (r *Registry) IsSealed() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sealed
}

// wrapBindingErr 用服务名与索引标注一个 binding 级错误，便于可操作的诊断。
func wrapBindingErr(service string, index int, err error) error {
	return &bindingError{service: service, index: index, err: err}
}

type bindingError struct {
	service string
	index   int
	err     error
}

func (e *bindingError) Error() string {
	return "ssrpc: binding " + itoa(e.index) + " of service " + e.service + ": " + e.err.Error()
}
func (e *bindingError) Unwrap() error { return e.err }
