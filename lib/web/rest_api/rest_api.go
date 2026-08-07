package rest_api

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	apiclient "github.com/Iori372552686/GoOne/lib/api/http_client"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"github.com/Iori372552686/GoOne/lib/web/http_sign"
)

// errNoSign 表示需要签名但该实例未装配 HttpSign。
var errNoSign = errors.New("rest_api: sign instance not configured")

// RestApi 是带可选签名与多后端分片的轻量出站 HTTP 客户端。
//
// 设计要点：
//   - 底层统一委托 lib/api/http_client.Client.DoRequest，天然获得 context 感知、
//     TLS 严格校验、响应体上限、非 2xx 错误处理；不再走 lib/web/http_client 垫层；
//   - 多 URL 采用 uin 取模分片，uin==0 时 atomic 自增轮询，并发安全且确定；
//   - 签名链路复用 http_sign：对调用方 query map 做防御性 clone，避免就地改写；
//   - 超时分层：ctx 自带 deadline 优先 → 实例 Timeout > 0 → 底层默认 8s。
type RestApi struct {
	serviceName string                   // 服务名，仅用于日志与错误归属
	urls        *urlPool                 // 后端地址池
	sign        *http_sign.HttpSign      // 可选签名实例
	client      *apiclient.Client        // 底层统一 HTTP 客户端
	timeout     time.Duration            // 兜底超时；0 表示复用底层默认
}

// Option 配置 RestApi，遵循 GoOne functional options 惯例（见 http_client.NewClient）。
type Option func(*RestApi)

// WithHTTPClient 注入自定义底层客户端，主要用于测试或共享特殊传输层。
// 传 nil 视作未设置，保留 NewClient(nil) 默认。
func WithHTTPClient(c *apiclient.Client) Option {
	return func(r *RestApi) {
		if c != nil {
			r.client = c
		}
	}
}

// WithTimeout 设置兜底超时；仅当调用方 ctx 无 deadline 时生效。
// d<=0 视作未设置，复用底层 Client 的默认超时。
func WithTimeout(d time.Duration) Option {
	return func(r *RestApi) {
		if d > 0 {
			r.timeout = d
		}
	}
}

// NewRestApi 依据 Config 构造 RestApi 实例。
//
// 必填：ServiceName 非空且 Urls 至少一项，否则返回 nil（由调用方决定是否报警）。
// signs 非 nil 且 conf.SignName 非空时，按名解析 HttpSign；解析不到则该实例不签名。
func NewRestApi(conf Config, signs *http_sign.SignMgr, opts ...Option) *RestApi {
	if conf.ServiceName == "" || len(conf.Urls) == 0 {
		return nil
	}

	r := &RestApi{
		serviceName: conf.ServiceName,
		urls:        newUrlPool(conf.Urls),
		client:      apiclient.NewClient(nil),
	}
	if signs != nil && conf.SignName != "" {
		r.sign = signs.GetSignIns(conf.SignName)
	}
	if conf.Timeout > 0 {
		r.timeout = time.Duration(conf.Timeout) * time.Second
	}
	for _, o := range opts {
		o(r)
	}

	logger.Infof("[%s] RestApi init | urls=%d sign=%t timeout=%s",
		conf.ServiceName, len(conf.Urls), r.sign != nil, r.timeout)
	return r
}

// newUrlPool 构造后端地址池；urls 为空时返回 nil（构造时已拦截，此处兜底）。
func newUrlPool(urls []string) *urlPool {
	if len(urls) == 0 {
		return nil
	}
	return &urlPool{urls: urls}
}

// urlPool 维护后端地址列表，并发安全地按 uin 取模分片或轮询。
type urlPool struct {
	urls []string
	seq  atomic.Uint64 // uin==0 时的轮询计数器
}

// pick 返回目标后端地址。uin!=0 时取模分片，uin==0 时 atomic 自增轮询。
func (p *urlPool) pick(uin int64) string {
	if len(p.urls) == 1 {
		return p.urls[0]
	}
	var idx uint64
	if uin != 0 {
		idx = uint64(uin) % uint64(len(p.urls))
	} else {
		idx = p.seq.Add(1) % uint64(len(p.urls))
	}
	return p.urls[idx]
}

// Get 发起普通 GET 请求，query 序列化进 URL；不做签名。
func (r *RestApi) Get(ctx context.Context, uin int64, query map[string]string) ([]byte, error) {
	return r.do(ctx, "GET", uin, query, nil, nil, false)
}

// SignedGet 发起带签名的 GET 请求：签名写入 query string（与历史报文格式一致）。
func (r *RestApi) SignedGet(ctx context.Context, uin int64, query map[string]string) ([]byte, error) {
	return r.do(ctx, "GET", uin, query, nil, nil, true)
}

// Post 发起普通 POST 请求；body 为原始字节（已是 JSON 序列化结果），headers 可为 nil。
func (r *RestApi) Post(ctx context.Context, uin int64, body []byte, headers map[string]string) ([]byte, error) {
	return r.do(ctx, "POST", uin, nil, body, headers, false)
}

// SignedPost 发起带签名的 POST 请求：
//   - query 走 query string 并参与签名；
//   - body 既作为签名内容也作为请求体；
//   - headers 透传（如 Authorization、Content-Type），不参与签名。
//
// query 会做防御性 clone，调用方原始 map 不被就地改写。
func (r *RestApi) SignedPost(ctx context.Context, uin int64, query map[string]string, body []byte, headers map[string]string) ([]byte, error) {
	return r.do(ctx, "POST", uin, query, body, headers, true)
}

// do 是所有请求方法的统一骨架。
//
// 出错统一返回 (nil, err)，绝不把 URL/部分 body 当作响应体返回（修复历史 Get 的缺陷）。
// 日志带上 serviceName 以便跨服务排障。
func (r *RestApi) do(ctx context.Context, method string, uin int64, query map[string]string, body []byte, headers map[string]string, sign bool) ([]byte, error) {
	if r == nil {
		return nil, errors.New("rest_api: nil receiver")
	}

	// 兜底超时：仅当调用方 ctx 无 deadline 且实例配置了 timeout 时包裹一层。
	if sign {
		if r.sign == nil {
			return nil, errNoSign
		}
		query = signedQuery(r.sign, query, body)
	}

	url := r.urls.pick(uin) + http_sign.Map2uri(query, "", true, false)

	ctx, cancel := applyTimeout(ctx, r.timeout)
	if cancel != nil {
		defer cancel()
	}

	resp, err := r.client.DoRequest(ctx, method, url, convert.Bytes2str(body), headers)
	if err != nil {
		logger.Errorf("[%s] %s request err | url=%s err=%v", r.serviceName, method, url, err)
		return nil, err
	}
	return resp, nil
}

// signedQuery 对 (clone(query) + body) 做签名，返回带 timestamp/sign 等字段的 query map。
//
// 防御性 clone：http_sign.PushSign 会就地写入时间戳与签名字段，
// 此处复制一份，保证调用方原始 map 不被改写（修复历史就地变异隐患）。
func signedQuery(sign *http_sign.HttpSign, query map[string]string, body []byte) map[string]string {
	q := make(map[string]string, len(query)+4)
	for k, v := range query {
		q[k] = v
	}
	return sign.PushSign(q, body)
}

// applyTimeout 在 ctx 无 deadline 且 d>0 时叠加 WithTimeout，返回新 ctx 与 cancel。
// cancel 为 nil 表示未包裹，调用方不应 defer。
func applyTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return ctx, nil
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, nil
	}
	return context.WithTimeout(ctx, d)
}
