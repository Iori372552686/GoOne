package bussvc

// 标准组件构造器（stdcomp.go）的单测。用 conf.LoadBytes 注入内存配置，
// 验证"服务名取 app.Name()、Start 时自读 conf"的装配契约。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/transaction"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/misc"
)

// testConfYAML 覆盖 CommonFromConf 读取的全部 key。admin 端口为 0，
// 用于验证 conf.ResolveAdminPort 按服务类型回退（connsvr→8101）。
const testConfYAML = `
connsvr:
  debug:
    log_dir: "./log/conn"
    log_level: "debug"
  identity:
    self_bus_id: "127.0.0.1:9001"
base_cfg:
  runtime:
    bus_mq_addr: "rabbitmq://guest:guest@127.0.0.1:5672/"
    register_addr: "127.0.0.1:2379"
    admin_server:
      enabled: false
      ip: "127.0.0.1"
      port: 0
    tracing:
      enabled: true
      exporter: "otlp"
      endpoint: "127.0.0.1:4317"
      insecure: true
      sampler_ratio: 0.5
  debug:
    pprof: true
`

// loadTestConf 每用例重置并注入内存配置（conf 启动不可变，跨用例必须 ResetForTest）。
func loadTestConf(t *testing.T, yaml string) {
	t.Helper()
	conf.ResetForTest()
	if err := conf.LoadBytes([]byte(yaml), ".yaml"); err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	t.Cleanup(conf.ResetForTest)
}

func newTestApp(t *testing.T, opts ...runtime.Option) *runtime.App {
	t.Helper()
	return runtime.MustNew("connsvr", opts...)
}

// TestLoadConf 验证标准加载闭包：从 conf.Path 发布快照、运行校验器、按序执行
// hooks、hook 错误带序号包装并中止。conf 启动不可变且 Load 读真实文件，
// 故每段用临时文件 + conf.Path 覆盖驱动（生产路径同款）。
func TestLoadConf(t *testing.T) {
	writeConf := func(t *testing.T, yaml string) {
		t.Helper()
		conf.ResetForTest()
		p := filepath.Join(t.TempDir(), "svr_conf.yaml")
		if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		old := conf.Path
		conf.Path = p
		t.Cleanup(func() { conf.Path = old; conf.ResetForTest() })
	}

	writeConf(t, `
testsvr:
  debug:
    log_dir: "./log"
`)
	var order []int
	load := LoadConf("testsvr",
		func(_ context.Context) error { order = append(order, 1); return nil },
		nil, // nil hook 被跳过
		func(_ context.Context) error { order = append(order, 2); return nil },
	)
	if err := load(context.Background()); err != nil {
		t.Fatalf("LoadConf: %v", err)
	}
	if !conf.Has("testsvr.debug.log_dir") {
		t.Fatal("conf snapshot not published")
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("hooks order = %v, want [1 2]", order)
	}

	// hook 失败：错误必须包装服务名与 hook 序号。
	writeConf(t, `testsvr: {}`)
	hookErr := errors.New("boom")
	load = LoadConf("testsvr", func(_ context.Context) error { return hookErr })
	if err := load(context.Background()); !errors.Is(err, hookErr) {
		t.Fatalf("LoadConf hook err = %v, want wrapped %v", err, hookErr)
	}
}

// TestWithConfLoader 验证 Option 形态：服务名自动取 app.Name()。
// 未注册校验器的服务名 RunValidators 放行，这里只断言加载成功且快照可读。
func TestWithConfLoader(t *testing.T) {
	loadTestConf(t, `
mysvc:
  debug:
    log_dir: "./log/mysvc"
`)
	app := runtime.MustNew("mysvc", WithConfLoader())
	if app == nil {
		t.Fatal("nil app")
	}
	// LoadConf 闭包行为已在 TestLoadConf 覆盖；这里验证 Option 装配不 panic 且
	// app 名正确传递（间接：NewLoggerComponent 读到 mysvc 段）。
	logComp := NewLoggerComponent(app)
	cfg := logComp.Cfg()
	if cfg.Name != "mysvc" || cfg.Dir != "./log/mysvc" {
		t.Fatalf("LoggerConfig = %+v, want Name=mysvc Dir=./log/mysvc", cfg)
	}
}

// TestMustNewRegistersStdComponents 验证 bussvc.MustNew 集中注册 4 个标准组件
// （datetime_tick → logger → admin → tracing），且 datetime_tick 排在首位。
// 这是各服务 app.go 不再手写这 4 行装配的契约保证。通过注入 ComponentTracker
// 读取注册顺序来断言。
func TestMustNewRegistersStdComponents(t *testing.T) {
	loadTestConf(t, `
mysvc:
  debug:
    log_dir: "./log/mysvc"
`)
	tracker := runtime.NewComponentTracker(nil)
	app := MustNew("mysvc", nil,
		runtime.WithComponentTracker(tracker),
		WithConfLoader(),
	)

	want := []string{"datetime_tick", "logger", "admin", "tracing"}
	reports := tracker.Report()
	if len(reports) < len(want) {
		t.Fatalf("期望至少 %d 个标准组件，实际 %d: %+v", len(want), len(reports), reports)
	}
	for i, name := range want {
		if reports[i].Name != name {
			t.Fatalf("第 %d 个组件 = %q, 期望 %q（完整顺序: %+v）", i, reports[i].Name, name, namesOf(reports))
		}
	}
	// app 本身可用。
	if app == nil || app.Name() != "mysvc" {
		t.Fatalf("app = %+v, want name mysvc", app)
	}
}

// namesOf 抽取组件报告的名字序列，便于失败信息展示。
func namesOf(reports []runtime.ComponentReport) []string {
	out := make([]string, len(reports))
	for i, r := range reports {
		out[i] = r.Name
	}
	return out
}

// TestNewLoggerComponent 验证 logger 组件从 conf 自读 dir/level，Name 为服务名。
func TestNewLoggerComponent(t *testing.T) {
	loadTestConf(t, testConfYAML)
	c := NewLoggerComponent(newTestApp(t)).Cfg()
	if c.Dir != "./log/conn" || c.Level != "debug" || c.Name != "connsvr" {
		t.Fatalf("LoggerConfig = %+v", c)
	}
}

// TestNewTracingComponent 验证 tracing 组件携带服务名并自读 base_cfg tracing 段。
func TestNewTracingComponent(t *testing.T) {
	loadTestConf(t, testConfYAML)
	comp := NewTracingComponent(newTestApp(t))
	if comp.ServiceName != "connsvr" {
		t.Fatalf("ServiceName = %q", comp.ServiceName)
	}
	cfg := comp.Cfg()
	if !cfg.Enabled || cfg.Exporter != "otlp" || cfg.Endpoint != "127.0.0.1:4317" ||
		!cfg.Insecure || cfg.SamplerRatio != 0.5 {
		t.Fatalf("TracingConfig = %+v", cfg)
	}
}

// TestNewAdminComponent 验证 admin 组件：Name 固定 "admin"；admin 段 disabled 时
// Start 不绑定监听器直接返回 nil（配置读取正确性的行为证据）；端口回退由
// CommonFromConf 单测覆盖（CommonFromConf("connsvr").AdminPort == 8101）。
func TestNewAdminComponent(t *testing.T) {
	loadTestConf(t, testConfYAML)
	app := newTestApp(t)
	probeCalled := false
	admin := NewAdminComponent(app, func() error { probeCalled = true; return nil })
	if admin.Name() != "admin" {
		t.Fatalf("Name = %q", admin.Name())
	}
	if err := admin.Start(context.Background()); err != nil {
		t.Fatalf("Start with disabled admin: %v", err)
	}
	if err := admin.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if probeCalled {
		t.Fatal("readyCheck 不应在 Start/Stop 路径被调用（仅供 /readyz）")
	}
	// admin 端口为 0 时按服务类型回退（connsvr→8101）。
	if got := CommonFromConf("connsvr").AdminPort; got != 8101 {
		t.Fatalf("AdminPort fallback = %d, want 8101", got)
	}
}

// TestNewRouterComponent 验证 router 组件：Common 自读 conf；TransMgr/Drivers 透传；
// 默认 OnRecvSSPacket 为 nil（Start 时回退 TransMgr.ProcessSSPacket）；传入时覆盖。
func TestNewRouterComponent(t *testing.T) {
	loadTestConf(t, testConfYAML)
	app := newTestApp(t)
	drivers := rabbitmq.NewRegistry()

	rc := NewRouterComponent(app, nil, drivers)
	if rc.TransMgr != nil || rc.Drivers != drivers || rc.OnRecvSSPacket != nil {
		t.Fatalf("RouterComponent = %+v", rc)
	}
	c := rc.Common()
	if c.SelfBusId != "127.0.0.1:9001" || c.RegisterAddr != "127.0.0.1:2379" ||
		c.BusMQAddr == "" || !c.Pprof {
		t.Fatalf("Common = %+v", c)
	}

	override := func(*sharedstruct.SSPacket) {}
	rc2 := NewRouterComponent(app, nil, drivers, override)
	if rc2.OnRecvSSPacket == nil {
		t.Fatal("OnRecvSSPacket 未按传入覆盖")
	}
}

// TestMustAppNamePanicsOnNil 验证 nil app 装配错误 fail-fast。
func TestMustAppNamePanicsOnNil(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewLoggerComponent(nil) 未 panic")
		}
	}()
	_ = NewLoggerComponent(nil)
}

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
// 全部在组件内部回退：InitAndRunWithConfig{MaxTrans: misc.MaxTransNumber,
// ShardCount: DefaultShardCount(), MaxPendingPerKey: DefaultMaxPendingPerKey}
//（connsvr/infosvr/mysqlsvr/mainsvr 的默认装配形态）。
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
		MaxTrans:         misc.MaxTransNumber,
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
		MaxTrans:         misc.MaxTransNumber,
		ShardCount:       transaction.DefaultShardCount(),
		MaxPendingPerKey: 200,
	}
	if mgr.gotCfg != want {
		t.Fatalf("cfg = %+v, want %+v", mgr.gotCfg, want)
	}
}
