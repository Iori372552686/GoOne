package bussvc

import (
	"context"
	"testing"

	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

// fakeTransMgr 只记录 Start 路径会触发的入口；其余方法经嵌入接口（nil）
// 兜底——组件若在 Start 调用它们会 panic，正是测试要抓的越界行为。
type fakeTransMgr struct {
	transaction.ITransactionMgr
	withCfgCalled bool
	gotCfg        transaction.TransactionMgrConfig
}

func (f *fakeTransMgr) InitAndRunWithConfig(cfg transaction.TransactionMgrConfig) {
	f.withCfgCalled = true
	f.gotCfg = cfg
}

// TestTransMgrComponentZeroConfigDefaults 验证全零配置走默认多分片路径，三项默认值
// 全部在组件内部回退：InitAndRunWithConfig{MaxTrans: defaultMaxTrans,
// ShardCount: DefaultShardCount(), MaxPendingPerKey: DefaultMaxPendingPerKey}
// （connsvr/infosvr/mysqlsvr/mainsvr 的默认装配形态）。
func TestTransMgrComponentZeroConfigDefaults(t *testing.T) {
	mgr := &fakeTransMgr{}
	c := &TransMgrComponent{Mgr: mgr}
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !mgr.withCfgCalled {
		t.Fatal("InitAndRunWithConfig 未被调用")
	}
	want := transaction.TransactionMgrConfig{
		MaxTrans:         defaultMaxTrans,
		ShardCount:       transaction.DefaultShardCount(),
		MaxPendingPerKey: transaction.DefaultMaxPendingPerKey,
	}
	if mgr.gotCfg != want {
		t.Fatalf("cfg = %+v, want %+v", mgr.gotCfg, want)
	}
}

// TestTransMgrComponentStaticCfg 验证显式 Cfg 原样透传（组件不做任何回退覆盖）。
func TestTransMgrComponentStaticCfg(t *testing.T) {
	mgr := &fakeTransMgr{}
	c := &TransMgrComponent{
		Mgr: mgr,
		Cfg: transaction.TransactionMgrConfig{MaxTrans: 5000, ShardCount: 4, MaxPendingPerKey: 200},
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := transaction.TransactionMgrConfig{MaxTrans: 5000, ShardCount: 4, MaxPendingPerKey: 200}
	if mgr.gotCfg != want {
		t.Fatalf("cfg = %+v, want %+v", mgr.gotCfg, want)
	}
}

// TestTransMgrComponentPartialCfg 验证只给 MaxPendingPerKey（roomcentersvr 形态）时
// 其余字段仍按默认回退。
func TestTransMgrComponentPartialCfg(t *testing.T) {
	mgr := &fakeTransMgr{}
	c := &TransMgrComponent{
		Mgr: mgr,
		Cfg: transaction.TransactionMgrConfig{MaxPendingPerKey: 200},
	}
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := transaction.TransactionMgrConfig{
		MaxTrans:         defaultMaxTrans,
		ShardCount:       transaction.DefaultShardCount(),
		MaxPendingPerKey: 200,
	}
	if mgr.gotCfg != want {
		t.Fatalf("cfg = %+v, want %+v", mgr.gotCfg, want)
	}
}
