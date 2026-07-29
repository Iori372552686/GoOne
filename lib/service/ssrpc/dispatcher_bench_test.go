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

// BenchmarkRegistrySeal100Bindings 测量 P1-01：Registry.Seal 100 个 binding 的成本。
func BenchmarkRegistrySeal100Bindings(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := NewRegistry()
		bindings := make([]Binding, 100)
		for j := 0; j < 100; j++ {
			bindings[j] = Binding{Kind: BindingCMD, CMD: g1_protocol.CMD(j + 1), CmdHandler: dummyCmdHandler()}
		}
		if err := r.Register("svc", bindings...); err != nil {
			b.Fatalf("register: %v", err)
		}
		if _, err := r.Seal(); err != nil {
			b.Fatalf("seal: %v", err)
		}
	}
}

// BenchmarkDispatcherCMDSealed 测量 sealed Dispatcher 的 WS 分发查找成本（0 alloc 目标）。
// CMD 分发经 TransactionMgr；这里用 DispatchWS 覆盖 sealed 只读 map 查找路径。
func BenchmarkDispatcherCMDSealed(b *testing.B) {
	r := NewRegistry()
	bindings := make([]Binding, 100)
	for j := 0; j < 100; j++ {
		bindings[j] = Binding{Kind: BindingWS, CMD: g1_protocol.CMD(j + 1), CmdHandler: dummyCmdHandler()}
	}
	if err := r.Register("svc", bindings...); err != nil {
		b.Fatalf("register: %v", err)
	}
	d, err := r.Seal()
	if err != nil {
		b.Fatalf("seal: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = d.DispatchWS(nil, uint32((i%100)+1), nil)
	}
}
