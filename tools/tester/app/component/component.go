package component

import (
	"context"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
	"github.com/golang/protobuf/proto"
)

type MessageSender interface {
	SendMessage(cmd uint32, req proto.Message) error
}

// Requester 同步请求-响应接口，由 session.Session 实现。
// 延迟与错误码统计在实现层自动完成。
type Requester interface {
	// Request 返回原始响应字节；req 为 nil 表示仅等待下一条 cmd 消息。
	Request(ctx context.Context, cmd uint32, req proto.Message, timeout time.Duration) ([]byte, error)
	// RequestProto 解码响应到 rsp，并自动提取业务错误码上报统计。
	RequestProto(ctx context.Context, cmd uint32, req, rsp proto.Message, timeout time.Duration) error
}

type TesterComponent interface {
	Name() string

	OnInit(ctx *ComponentContext) error

	OnConnected() error

	OnAccountLogin(accountID string) error

	OnRoleLogin(userID int64) error

	RunTests(ctx context.Context) error

	OnMessage(cmd uint32, data []byte) bool
}

// StressRunner 组件可选实现：暴露一轮"仅正常路径"的业务操作供全流程压测循环调用。
// 与 RunTests（完整回归用例集，含边界/错误用例）分离，避免污染压测错误码统计。
type StressRunner interface {
	RunStress(ctx context.Context) error
}

type ComponentContext struct {
	ActorID   int
	AccountID string
	UserID    int64
	Sender    MessageSender
	Requester Requester
	// Cfg 统一测试配置；组件细项参数用 Cfg.DecodeModule(name, &v) 解码。
	Cfg *testcfg.Config
}
