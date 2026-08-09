package transaction

import (
	"sync/atomic"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// BenchmarkTransactionMgrThroughput measures the end-to-end dispatch cost of
// the transaction manager with a no-op handler: shard selection, serial-key
// bookkeeping, per-request goroutine spawn and completion. This is the
// baseline for docs/optimization_roadmap.md phase 3.7 (worker-pool prototype).
func BenchmarkTransactionMgrThroughput(b *testing.B) {
	benchTransactionMgr(b, 4, false)
}

// BenchmarkTransactionMgrSerialKey measures the same path with uid
// serialization enabled and all packets sharing one uid, i.e. the fully
// serialized worst case (pending queue churn).
func BenchmarkTransactionMgrSerialKey(b *testing.B) {
	benchTransactionMgr(b, 1, true)
}

func benchTransactionMgr(b *testing.B, shards int, sameUID bool) {
	mgr := &TransactionMgr{}
	var done atomic.Int64

	mgr.RegisterCmd(testTransactionCmd, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		done.Add(1)
		return g1_protocol.ErrorCode_ERR_OK
	})
	mgr.InitAndRunWithConfig(TransactionMgrConfig{
		MaxTrans:         int32(b.N + 1),
		ShardCount:       shards,
		MaxPendingPerKey: b.N + 1,
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uid := uint64(10000)
		if !sameUID {
			uid += uint64(i % 1024)
		}
		mgr.ProcessSSPacket(&sharedstruct.SSPacket{
			Header: sharedstruct.SSPacketHeader{
				Uid: uid,
				Cmd: uint32(testTransactionCmd),
			},
		})
	}

	// Wait for all handlers to complete so the measurement covers execution.
	for done.Load() < int64(b.N) {
	}
	b.StopTimer()
}
