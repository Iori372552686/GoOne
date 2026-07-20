package ssrpc

import (
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// BenchmarkDispatchWS_Sealed 度量已 Seal dispatcher 上 WS 查找的热路径（无锁只读
// map 读）。这是生产中使用的 Seal 后路径。与 BenchmarkDispatchWS_Unsealed 对比，以
// 确认热路径已移除 RWMutex（P0-04 / P1-02 门禁）。
func BenchmarkDispatchWS_Sealed(b *testing.B) {
	d := NewDispatcher()
	d.RegisterWS(1, func(cmd_handler.IContext, []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	})
	d.Seal()
	body := []byte("x")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.DispatchWS(nil, 1, body)
	}
}

// BenchmarkDispatchWS_Unsealed 是遗留的每调用 RLock 路径，作为回归基线保留。Sealed
// 变体必须至少同样快且分配不多于它。
func BenchmarkDispatchWS_Unsealed(b *testing.B) {
	d := NewDispatcher()
	d.RegisterWS(1, func(cmd_handler.IContext, []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	})
	body := []byte("x")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = d.DispatchWS(nil, 1, body)
	}
}
