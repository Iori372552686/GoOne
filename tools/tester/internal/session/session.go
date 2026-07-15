// Package session 模拟客户端会话层：TCP/WebSocket 连接、CSPacket 编解码、同步请求-响应、登录流程。
//
// 回归测试与压力测试共用本层；协议级延迟与错误码在 Request 内自动上报 stats.Collector，
// 业务组件无需关心统计埋点。
package session

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/tools/tester/internal/stats"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
)

// DefaultRequestTimeout 单次请求默认超时。
const DefaultRequestTimeout = 30 * time.Second

// PushHandler 观察收到的每条消息（响应与推送）；返回 true 表示已消费（仅作日志语义，
// 不影响 pending 请求的匹配）。
type PushHandler func(cmd uint32, data []byte) bool

// Options 会话构造参数。
type Options struct {
	ID int // 模拟玩家编号

	Transport string // tcp | ws
	Host      string
	TcpPort   int
	WsPort    int
	WsPath    string

	Channel   string
	AccountID string
	DeviceID  string
	UserID    int64
	Token     string

	Collector      *stats.Collector // 可为 nil（不统计）
	ConnectTimeout time.Duration
}

type waiter struct {
	ch chan []byte
}

// Session 单个模拟玩家的网络会话。
type Session struct {
	opts Options

	tcpConn  net.Conn
	wsConn   *websocket.Conn
	seq      atomic.Uint32
	uid      atomic.Uint64

	pendingMu sync.Mutex
	pending   map[stats.ProtoKey][]*waiter

	handlersMu sync.RWMutex
	handlers   []PushHandler

	connectedCh  chan struct{}
	connectOnce  sync.Once
	closed       atomic.Bool
	disconnected atomic.Bool

	module atomic.Value // string：当前业务模块名（统计归属）

	accountID string
	userID    int64
	roleName  string
}

func New(opts Options) *Session {
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 10 * time.Second
	}
	if opts.Transport == "" {
		opts.Transport = "tcp"
	}
	s := &Session{
		opts:        opts,
		pending:     make(map[stats.ProtoKey][]*waiter),
		connectedCh: make(chan struct{}),
		accountID:   opts.AccountID,
		userID:      opts.UserID,
	}
	s.module.Store("core")
	return s
}

func (s *Session) ID() int           { return s.opts.ID }
func (s *Session) AccountID() string { return s.accountID }
func (s *Session) UserID() int64     { return s.userID }
func (s *Session) RoleName() string  { return s.roleName }

// SetModule 设置后续请求的统计归属模块。
func (s *Session) SetModule(name string) {
	if name == "" {
		name = "core"
	}
	s.module.Store(name)
}

func (s *Session) currentModule() string {
	if v, ok := s.module.Load().(string); ok {
		return v
	}
	return "core"
}

// OnMessage 注册消息观察者（组件缓存推送数据用）。
func (s *Session) OnMessage(fn PushHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers = append(s.handlers, fn)
}

// Connected 报告底层连接是否仍然存活。
func (s *Session) Connected() bool {
	if s.opts.Transport == "ws" {
		return s.wsConn != nil && !s.disconnected.Load()
	}
	return s.tcpConn != nil && !s.disconnected.Load()
}

// Connect 建立连接并等待握手完成。
func (s *Session) Connect(ctx context.Context) error {
	switch s.opts.Transport {
	case "ws":
		return s.connectWS(ctx)
	default:
		return s.connectTCP(ctx)
	}
}

func (s *Session) connectTCP(ctx context.Context) error {
	addr := net.JoinHostPort(s.opts.Host, strconv.Itoa(s.opts.TcpPort))
	conn, err := net.DialTimeout("tcp", addr, s.opts.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("session %d: dial tcp %s: %w", s.opts.ID, addr, err)
	}
	s.tcpConn = conn
	s.disconnected.Store(false)
	s.connectOnce.Do(func() { close(s.connectedCh) })

	go s.readLoopTCP()
	return nil
}

func (s *Session) connectWS(ctx context.Context) error {
	path := s.opts.WsPath
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", s.opts.Host, s.opts.WsPort), Path: path}

	dialer := websocket.Dialer{HandshakeTimeout: s.opts.ConnectTimeout}
	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("session %d: dial ws %s: %w", s.opts.ID, u.String(), err)
	}
	s.wsConn = conn
	s.disconnected.Store(false)
	s.connectOnce.Do(func() { close(s.connectedCh) })

	go s.readLoopWS()
	return nil
}

// Close 关闭连接。
func (s *Session) Close() {
	if s.closed.Swap(true) {
		return
	}
	if s.tcpConn != nil {
		_ = s.tcpConn.Close()
	}
	if s.wsConn != nil {
		_ = s.wsConn.Close()
	}
}

// ---------------------------------------------------------------------------
// 收发
// ---------------------------------------------------------------------------

func (s *Session) readLoopTCP() {
	conn := s.tcpConn
	headerBuf := make([]byte, sharedstruct.ByteLenOfCSPacketHeader())
	for {
		if s.closed.Load() {
			return
		}
		if _, err := io.ReadFull(conn, headerBuf); err != nil {
			s.onDisconnect()
			return
		}
		header := sharedstruct.CSPacketHeader{}
		header.From(headerBuf)

		body := make([]byte, header.BodyLen)
		if header.BodyLen > 0 {
			if _, err := io.ReadFull(conn, body); err != nil {
				s.onDisconnect()
				return
			}
		}
		s.handleMessage(header.Cmd, body)
	}
}

func (s *Session) readLoopWS() {
	conn := s.wsConn
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			s.onDisconnect()
			return
		}
		if len(data) < sharedstruct.ByteLenOfCSPacketHeader() {
			continue
		}
		header := sharedstruct.CSPacketHeader{}
		header.From(data)
		body := data[sharedstruct.ByteLenOfCSPacketHeader():]
		if len(body) < int(header.BodyLen) {
			continue
		}
		body = body[:header.BodyLen]
		s.handleMessage(header.Cmd, body)
	}
}

func (s *Session) onDisconnect() {
	s.disconnected.Store(true)
}

func (s *Session) handleMessage(cmd uint32, data []byte) {
	// 1) 观察者（组件缓存推送/响应数据）
	s.handlersMu.RLock()
	handlers := s.handlers
	s.handlersMu.RUnlock()
	for _, h := range handlers {
		if h(cmd, data) {
			break
		}
	}

	// 2) 唤醒 pending 请求（FIFO）
	key := stats.ProtoKey{Cmd: cmd}
	s.pendingMu.Lock()
	queue := s.pending[key]
	var w *waiter
	if len(queue) > 0 {
		w = queue[0]
		if len(queue) == 1 {
			delete(s.pending, key)
		} else {
			s.pending[key] = queue[1:]
		}
	}
	s.pendingMu.Unlock()

	if w != nil {
		select {
		case w.ch <- data:
		default:
		}
	}
}

// Send 单向发送（不等待响应，不统计延迟）。
func (s *Session) Send(cmd uint32, req proto.Message) error {
	return s.sendMessage(cmd, req)
}

// SendMessage 兼容 component.MessageSender 接口。
func (s *Session) SendMessage(cmd uint32, req proto.Message) error {
	return s.sendMessage(cmd, req)
}

func (s *Session) sendMessage(cmd uint32, req proto.Message) error {
	var body []byte
	var err error
	if req != nil {
		body, err = proto.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      s.seq.Add(1),
		Uid:      s.uid.Load(),
		Cmd:      cmd,
		BodyLen:  uint32(len(body)),
	}
	headerBytes := header.ToBytes()

	if s.opts.Transport == "ws" {
		if s.wsConn == nil || s.disconnected.Load() {
			return fmt.Errorf("not connected")
		}
		return s.wsConn.WriteMessage(websocket.BinaryMessage, append(headerBytes, body...))
	}

	if s.tcpConn == nil || s.disconnected.Load() {
		return fmt.Errorf("not connected")
	}
	if _, err := s.tcpConn.Write(headerBytes); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if len(body) > 0 {
		if _, err := s.tcpConn.Write(body); err != nil {
			return fmt.Errorf("write body: %w", err)
		}
	}
	return nil
}

func (s *Session) registerWaiter(key stats.ProtoKey) *waiter {
	w := &waiter{ch: make(chan []byte, 1)}
	s.pendingMu.Lock()
	s.pending[key] = append(s.pending[key], w)
	s.pendingMu.Unlock()
	return w
}

func (s *Session) removeWaiter(key stats.ProtoKey, w *waiter) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	queue := s.pending[key]
	for i, item := range queue {
		if item == w {
			queue = append(queue[:i], queue[i+1:]...)
			break
		}
	}
	if len(queue) == 0 {
		delete(s.pending, key)
	} else {
		s.pending[key] = queue
	}
}

// Request 同步请求-响应，返回原始响应字节。
//   - req 为 nil 时表示只等待下一条 cmd 消息（等推送），不发送、不统计。
//
// GoOne 约定响应 cmd = 请求 cmd + 1（REQ/RSP 配对，见 ssrpc sendback 的 cmd+1 convention）。
// waiter 以响应 cmd 为 key 注册，与 handleMessage 收到的响应包 header.Cmd 匹配。
func (s *Session) Request(ctx context.Context, cmd uint32, req proto.Message, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	key := stats.ProtoKey{Cmd: rspCmdOf(cmd)}
	w := s.registerWaiter(key)

	start := time.Now()
	if req != nil {
		if err := s.sendMessage(cmd, req); err != nil {
			s.removeWaiter(key, w)
			s.record(key, req, 0, stats.OutcomeSendFail, 0)
			return nil, fmt.Errorf("send cmd=0x%x: %w", cmd, err)
		}
	}

	select {
	case data := <-w.ch:
		if req != nil {
			s.record(key, req, time.Since(start), stats.OutcomeSuccess, 0)
		}
		return data, nil
	case <-ctx.Done():
		s.removeWaiter(key, w)
		return nil, ctx.Err()
	case <-time.After(timeout):
		s.removeWaiter(key, w)
		if req != nil {
			s.record(key, req, time.Since(start), stats.OutcomeTimeout, 0)
		}
		return nil, fmt.Errorf("timeout waiting for response (cmd=0x%x)", cmd)
	}
}

// RequestProto 同步请求-响应并解码到 rsp；自动提取业务错误码上报统计。
// 返回值只反映传输层结果；业务错误码非 0 不视为 error，由调用方检查 rsp。
func (s *Session) RequestProto(ctx context.Context, cmd uint32, req, rsp proto.Message, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = DefaultRequestTimeout
	}
	key := stats.ProtoKey{Cmd: rspCmdOf(cmd)}
	w := s.registerWaiter(key)

	start := time.Now()
	if req != nil {
		if err := s.sendMessage(cmd, req); err != nil {
			s.removeWaiter(key, w)
			s.record(key, req, 0, stats.OutcomeSendFail, 0)
			return fmt.Errorf("send cmd=0x%x: %w", cmd, err)
		}
	}

	select {
	case data := <-w.ch:
		rtt := time.Since(start)
		if rsp != nil {
			if err := proto.Unmarshal(data, rsp); err != nil {
				s.record(key, req, rtt, stats.OutcomeSuccess, 0)
				return fmt.Errorf("unmarshal response (cmd=0x%x): %w", cmd, err)
			}
		}
		code := ExtractErrCode(rsp)
		if IsErrCode(code) {
			s.record(key, req, rtt, stats.OutcomeBizError, code)
		} else {
			s.record(key, req, rtt, stats.OutcomeSuccess, 0)
		}
		return nil
	case <-ctx.Done():
		s.removeWaiter(key, w)
		return ctx.Err()
	case <-time.After(timeout):
		s.removeWaiter(key, w)
		s.record(key, req, time.Since(start), stats.OutcomeTimeout, 0)
		return fmt.Errorf("timeout waiting for response (cmd=0x%x)", cmd)
	}
}

func (s *Session) record(key stats.ProtoKey, req proto.Message, rtt time.Duration, outcome stats.Outcome, code int32) {
	if s.opts.Collector == nil {
		return
	}
	s.opts.Collector.RecordRequest(key, s.currentModule(), protoName(req), rtt, outcome, code)
}

func protoName(req proto.Message) string {
	if req == nil {
		return ""
	}
	name := proto.MessageName(req)
	// 去掉包名前缀，如 "game.LoginReq" -> "LoginReq"
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			return name[i+1:]
		}
	}
	return name
}

// rspCmdOf 按 GoOne 的 cmd+1 约定返回响应 cmd。
// 请求/响应在 cmd 枚举里成对定义（REQ=偶数, RSP=REQ+1），服务端回包 header.Cmd 即此值。
// 对已经是响应 cmd（奇数）的入参保持不变，避免重复 +1。
func rspCmdOf(reqCmd uint32) uint32 {
	if reqCmd&1 == 1 {
		return reqCmd // 已是响应 cmd（奇数），直接用
	}
	return reqCmd + 1
}
