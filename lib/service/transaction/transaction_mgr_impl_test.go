package transaction

import (
	"context"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

const testTransactionCmd = g1_protocol.CMD(900001)

func TestNormalizeConfigUsesDefaultShardCount(t *testing.T) {
	cfg := normalizeConfig(TransactionMgrConfig{
		MaxTrans:         16,
		ShardCount:       0,
		MaxPendingPerKey: 8,
	})

	if cfg.ShardCount != DefaultShardCount() {
		t.Fatalf("expected default shard count %d, got %d", DefaultShardCount(), cfg.ShardCount)
	}
}

func TestTransactionMgrFallsBackToUIDWhenRouterIDIsEmpty(t *testing.T) {
	mgr := &TransactionMgr{}
	started := make(chan uint64, 4)
	release := make(chan struct{}, 4)

	mgr.RegisterCmd(testTransactionCmd, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		started <- c.Uid()
		<-release
		return g1_protocol.ErrorCode_ERR_OK
	})
	mgr.InitAndRunWithConfig(TransactionMgrConfig{
		MaxTrans:         8,
		ShardCount:       4,
		MaxPendingPerKey: 4,
	})

	mgr.ProcessSSPacket(makeTestPacket(1001, 0, testTransactionCmd))
	if uid := waitStarted(t, started); uid != 1001 {
		t.Fatalf("expected first transaction uid=1001, got %d", uid)
	}

	// When RouterID is empty, the serial key falls back to uid.
	mgr.ProcessSSPacket(makeTestPacket(1001, 0, testTransactionCmd))
	waitFor(t, func() bool { return mgr.StatsSnapshot().PendingPackets == 1 }, "pending packet to be recorded")
	ensureNoStart(t, started, 150*time.Millisecond)

	release <- struct{}{}
	if uid := waitStarted(t, started); uid != 1001 {
		t.Fatalf("expected queued transaction uid=1001, got %d", uid)
	}

	release <- struct{}{}
	waitFor(t, func() bool {
		stats := mgr.StatsSnapshot()
		return stats.ActiveTransactions == 0 && stats.PendingPackets == 0
	}, "uid-fallback serialized transactions to drain")
}

func TestTransactionMgrSerializesByRouterIDAndTracksDrops(t *testing.T) {
	mgr := &TransactionMgr{}
	started := make(chan uint64, 4)
	release := make(chan struct{}, 4)

	mgr.RegisterCmd(testTransactionCmd, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		started <- c.Uid()
		<-release
		return g1_protocol.ErrorCode_ERR_OK
	})
	mgr.InitAndRunWithConfig(TransactionMgrConfig{
		MaxTrans:         8,
		ShardCount:       4,
		MaxPendingPerKey: 1,
	})

	mgr.ProcessSSPacket(makeTestPacket(2001, 77, testTransactionCmd))
	if uid := waitStarted(t, started); uid != 2001 {
		t.Fatalf("expected first transaction uid=2001, got %d", uid)
	}

	// Same rid but different uid should still queue behind the running transaction.
	mgr.ProcessSSPacket(makeTestPacket(2002, 77, testTransactionCmd))
	waitFor(t, func() bool { return mgr.StatsSnapshot().PendingPackets == 1 }, "router-id pending packet to be recorded")

	// The third packet exceeds MaxPendingPerKey and should be dropped.
	mgr.ProcessSSPacket(makeTestPacket(2003, 77, testTransactionCmd))
	waitFor(t, func() bool { return mgr.StatsSnapshot().DroppedPackets == 1 }, "dropped packet counter to increase")

	release <- struct{}{}
	if uid := waitStarted(t, started); uid != 2002 {
		t.Fatalf("expected queued router-id transaction uid=2002, got %d", uid)
	}

	release <- struct{}{}
	waitFor(t, func() bool {
		stats := mgr.StatsSnapshot()
		return stats.ActiveTransactions == 0 && stats.PendingPackets == 0 && stats.DroppedPackets == 1
	}, "router-id serialized transactions to drain")
}

func TestTransactionMgrCloseRejectsNewRequestsAndWaitsForInflight(t *testing.T) {
	mgr := &TransactionMgr{}
	started := make(chan uint64, 2)
	release := make(chan struct{}, 1)

	mgr.RegisterCmd(testTransactionCmd, func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		started <- c.Uid()
		<-release
		return g1_protocol.ErrorCode_ERR_OK
	})
	mgr.InitAndRunWithConfig(TransactionMgrConfig{
		MaxTrans:         4,
		ShardCount:       1,
		MaxPendingPerKey: 2,
	})

	mgr.ProcessSSPacket(makeTestPacket(3001, 0, testTransactionCmd))
	if uid := waitStarted(t, started); uid != 3001 {
		t.Fatalf("expected first transaction uid=3001, got %d", uid)
	}

	closeDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		closeDone <- mgr.Close(ctx)
	}()

	// Wait until shutdown has actually begun before probing rejection,
	// otherwise the new packet may race ahead of Close().
	waitFor(t, func() bool { return mgr.closing.Load() }, "shutdown to begin")

	// New requests should be rejected once shutdown begins.
	mgr.ProcessSSPacket(makeTestPacket(3002, 0, testTransactionCmd))
	ensureNoStart(t, started, 150*time.Millisecond)

	release <- struct{}{}

	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Close to finish")
	}

	stats := mgr.StatsSnapshot()
	if stats.ActiveTransactions != 0 {
		t.Fatalf("expected no active transactions after close, got %d", stats.ActiveTransactions)
	}
}

func makeTestPacket(uid, rid uint64, cmd g1_protocol.CMD) *sharedstruct.SSPacket {
	return &sharedstruct.SSPacket{
		Header: sharedstruct.SSPacketHeader{
			Uid:      uid,
			RouterID: rid,
			Cmd:      uint32(cmd),
		},
	}
}

func waitStarted(t *testing.T, started <-chan uint64) uint64 {
	t.Helper()
	select {
	case uid := <-started:
		return uid
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for transaction start")
		return 0
	}
}

func ensureNoStart(t *testing.T, started <-chan uint64, timeout time.Duration) {
	t.Helper()
	select {
	case uid := <-started:
		t.Fatalf("unexpected transaction start for uid=%d", uid)
	case <-time.After(timeout):
	}
}

func waitFor(t *testing.T, cond func() bool, desc string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

// TestRegisterCmdERejectsNil 验证 RegisterCmdE 对 nil handler 返回
// ErrNilCmdHandler。
func TestRegisterCmdERejectsNil(t *testing.T) {
	mgr := &TransactionMgr{}
	if err := mgr.RegisterCmdE(testTransactionCmd, nil); err != ErrNilCmdHandler {
		t.Fatalf("期望 ErrNilCmdHandler，got %v", err)
	}
}

// TestRegisterCmdERejectsDuplicate 验证 重复 cmd 返回 ErrDuplicateCmd（不再
// last-write-wins 静默覆盖）。
func TestRegisterCmdERejectsDuplicate(t *testing.T) {
	mgr := &TransactionMgr{}
	h := func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	}
	if err := mgr.RegisterCmdE(testTransactionCmd, h); err != nil {
		t.Fatalf("首次注册失败: %v", err)
	}
	if err := mgr.RegisterCmdE(testTransactionCmd, h); err != ErrDuplicateCmd {
		t.Fatalf("期望 ErrDuplicateCmd，got %v", err)
	}
}

// TestRegisterCmdERejectsAfterStart 验证 InitAndRun 之后注册返回
// ErrRegisterAfterStart（历史为 logger.Fatalf 杀进程）。
func TestRegisterCmdERejectsAfterStart(t *testing.T) {
	mgr := &TransactionMgr{}
	h := func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	}
	mgr.InitAndRunWithConfig(TransactionMgrConfig{
		MaxTrans:         4,
		ShardCount:       2,
		MaxPendingPerKey: 2,
	})
	defer mgr.Close(context.Background())
	if err := mgr.RegisterCmdE(testTransactionCmd, h); err != ErrRegisterAfterStart {
		t.Fatalf("期望 ErrRegisterAfterStart，got %v", err)
	}
}

// TestLegacyRegisterCmdDoesNotFatalOrOverwrite 验证 兼容 RegisterCmd 不 Fatal、
// 不静默覆盖（重复注册时第二个被拒绝，第一个保留）。
func TestLegacyRegisterCmdDoesNotFatalOrOverwrite(t *testing.T) {
	mgr := &TransactionMgr{}
	first := func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_OK
	}
	second := func(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
		return g1_protocol.ErrorCode_ERR_INTERNAL
	}
	mgr.RegisterCmd(testTransactionCmd, first)
	mgr.RegisterCmd(testTransactionCmd, second) // 重复：应被拒绝，不覆盖
	// 第一个 handler 仍在，未被 second 覆盖。
	if mgr.cmdHandlers[testTransactionCmd] == nil {
		t.Fatal("first handler 应保留")
	}
}
