package role

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/db/dal"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	goredis "github.com/redis/go-redis/v9"
)

// ---- 本文件补充 DAL 链路的 P0/P1 缺口用例（代码测验维度：数据一致性/边界） ----
// 桩均在本地定义，避免与 currency_test.go 的 fakeRoleL3 纠缠。

// retryL3 可控失败的 L3 桩（投递重试语义验证）。
type retryL3 struct {
	calls int
	err   error
	seen  [][]byte
}

func (f *retryL3) Load(context.Context, uint64) ([]byte, int64, error) { return nil, 0, nil }
func (f *retryL3) Save(_ context.Context, _ uint64, data []byte, _ int64) error {
	f.calls++
	f.seen = append(f.seen, data)
	return f.err
}
func (f *retryL3) Delete(context.Context, uint64) error { return nil }

func withRetryL3(t *testing.T, err error) *retryL3 {
	t.Helper()
	l3 := &retryL3{err: err}
	old := roleStore
	roleStore = NewRoleStore(func() int64 { return 8888 })
	roleStore.setL3(l3)
	t.Cleanup(func() { roleStore = old })
	return l3
}

// hashStub 最小 Redis 桩（角色 L2 用到的 HGetAll/HSet/Expire/Del）。
type hashStub struct {
	goredis.UniversalClient
	data map[string]map[string]string
}

func (c *hashStub) HGetAll(_ context.Context, key string) *goredis.MapStringStringCmd {
	cmd := goredis.NewMapStringStringCmd(context.Background())
	m := map[string]string{}
	for f, v := range c.data[key] {
		m[f] = v
	}
	cmd.SetVal(m)
	return cmd
}

func (c *hashStub) HSet(_ context.Context, key string, values ...interface{}) *goredis.IntCmd {
	if m, ok := values[0].(map[string]interface{}); ok {
		if c.data[key] == nil {
			c.data[key] = map[string]string{}
		}
		for f, v := range m {
			c.data[key][f] = string(v.([]byte))
		}
	}
	return goredis.NewIntResult(1, nil)
}

func (c *hashStub) Expire(context.Context, string, time.Duration) *goredis.BoolCmd {
	return goredis.NewBoolResult(true, nil)
}

func (c *hashStub) Del(_ context.Context, keys ...string) *goredis.IntCmd {
	for _, k := range keys {
		delete(c.data, k)
	}
	return goredis.NewIntResult(1, nil)
}

// hashPipe 配合 hashStub 的 TxPipeline 桩（TTL 路径：HSET+EXPIRE 经管道提交）。
type hashPipe struct {
	goredis.Pipeliner
	stub *hashStub
}

func (c *hashStub) TxPipeline() goredis.Pipeliner { return &hashPipe{stub: c} }

func (p *hashPipe) HSet(ctx context.Context, key string, values ...interface{}) *goredis.IntCmd {
	return p.stub.HSet(ctx, key, values...)
}

func (p *hashPipe) Expire(context.Context, string, time.Duration) *goredis.BoolCmd {
	return goredis.NewBoolResult(true, nil)
}

func (p *hashPipe) Exec(context.Context) ([]goredis.Cmder, error) { return nil, nil }

func withHashStub(t *testing.T) *hashStub {
	t.Helper()
	client := &hashStub{data: map[string]map[string]string{}}
	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	rds.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_ROLE), client)
	t.Cleanup(func() { rds.RedisMgr = oldMgr })
	return client
}

// 防抖窗口内（force=false）不投递、标记保留。
func TestMaybeFlushL3DebounceSuppresses(t *testing.T) {
	l3 := withRetryL3(t, nil)
	r := NewRole(710001)
	r.needL3Flush = true
	r.lastL3FlushAt = r.Now()

	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatalf("窗口内应静默跳过: %v", err)
	}
	if l3.calls != 0 {
		t.Fatalf("防抖窗口内不应投递，实际 %d 次", l3.calls)
	}
	if !r.needL3Flush {
		t.Fatal("跳过投递不应清除标记")
	}
}

// lastL3FlushAt==0（从未投递）同样受防抖约束（组件化后的新语义）。
func TestMaybeFlushL3ZeroTimestampDebounced(t *testing.T) {
	l3 := withRetryL3(t, nil)
	r := NewRole(710002)
	r.needL3Flush = true
	r.lastL3FlushAt = 0

	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatal(err)
	}
	if l3.calls != 0 {
		t.Fatalf("lastL3FlushAt==0 应防抖，实际投递 %d 次", l3.calls)
	}
}

// 防抖窗口外正常投递并清标记。
func TestMaybeFlushL3DebounceElapsedSends(t *testing.T) {
	l3 := withRetryL3(t, nil)
	r := NewRole(710003)
	r.needL3Flush = true
	r.lastL3FlushAt = r.Now() - 61 // 默认防抖 60s

	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatalf("窗口外应投递: %v", err)
	}
	if l3.calls != 1 {
		t.Fatalf("应投递 1 次，实际 %d", l3.calls)
	}
	if r.needL3Flush {
		t.Fatal("投递成功后标记应清除")
	}
}

// force 无视防抖立即投递。
func TestMaybeFlushL3ForceBypassesDebounce(t *testing.T) {
	l3 := withRetryL3(t, nil)
	r := NewRole(710004)
	r.needL3Flush = true
	r.lastL3FlushAt = r.Now()

	if err := r.MaybeFlushL3(true); err != nil {
		t.Fatalf("force 应投递: %v", err)
	}
	if l3.calls != 1 {
		t.Fatalf("force 应立即投递，实际 %d 次", l3.calls)
	}
}

// 投递失败保留标记；恢复后重试成功清除（mysqlsvr 瞬断自愈语义）。
func TestMaybeFlushL3SendFailureRetainsFlag(t *testing.T) {
	l3 := withRetryL3(t, errors.New("bus down"))
	r := NewRole(710005)
	r.needL3Flush = true
	r.lastL3FlushAt = r.Now() - 61

	if err := r.MaybeFlushL3(false); err == nil {
		t.Fatal("投递失败必须返回 error")
	}
	if !r.needL3Flush {
		t.Fatal("失败后标记应保留供重试")
	}

	l3.err = nil
	r.lastL3FlushAt = r.Now() - 61
	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatalf("恢复后应成功: %v", err)
	}
	if r.needL3Flush {
		t.Fatal("重试成功后标记应清除")
	}
}

// 无待写标记时 force 也不投递（幂等，避免空快照）。
func TestMaybeFlushL3NoFlagNoop(t *testing.T) {
	l3 := withRetryL3(t, nil)
	r := NewRole(710006)
	if err := r.MaybeFlushL3(true); err != nil {
		t.Fatal(err)
	}
	if l3.calls != 0 {
		t.Fatalf("无标记不应投递，实际 %d 次", l3.calls)
	}
}

// roleCacheTTL：未配置=0（现状兼容）；配置 N 天=N*24h。
func TestRoleCacheTTLConfig(t *testing.T) {
	if got := roleCacheTTL(); got != 0 {
		t.Fatalf("未配置时 TTL 应为 0，got %v", got)
	}
	if err := conf.LoadBytes([]byte("mainsvr:\n  capacity:\n    role_cache_ttl_days: 30\n"), ".yaml"); err != nil {
		t.Fatal(err)
	}
	if got, want := roleCacheTTL(), 30*24*time.Hour; got != want {
		t.Fatalf("配置 30 天时 TTL=%v want %v", got, want)
	}
}

// codec 货币段往返：金币已迁 currency_map，L2 分段与 L3 整包都必须保留。
// 用零默认值币种（LIVENESS/STAMINA）断言，避免初值礼包干扰。
func TestRoleCodecPreservesCurrencySection(t *testing.T) {
	r := NewRole(710007)
	r.Currency.Add(int32(g1_protocol.EItemID_LIVENESS), 777, nil)
	r.Currency.Add(int32(g1_protocol.EItemID_STAMINA), 88, nil)

	var codec roleCodec
	blob, err := codec.MarshalL3(r.PbRole)
	if err != nil {
		t.Fatal(err)
	}
	back, err := codec.UnmarshalL3(blob)
	if err != nil {
		t.Fatal(err)
	}
	cm := back.GetCurrencyInfo().GetCurrencyMap()
	if cm[int32(g1_protocol.EItemID_LIVENESS)] != 777 || cm[int32(g1_protocol.EItemID_STAMINA)] != 88 {
		t.Fatalf("L3 往返货币丢失: %v", cm)
	}

	fields, err := codec.MarshalL2Fields(r.PbRole, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["currency"]; !ok {
		t.Fatalf("全量 L2 应含 currency 段，实际字段: %v", fields)
	}
	back2, err := codec.UnmarshalL2Fields(fields)
	if err != nil {
		t.Fatal(err)
	}
	if back2.GetCurrencyInfo().GetCurrencyMap()[int32(g1_protocol.EItemID_LIVENESS)] != 777 {
		t.Fatalf("L2 往返货币丢失: %v", back2.GetCurrencyInfo().GetCurrencyMap())
	}
}

// 双 uid 数据隔离：L2 写入/读回互不串扰。
func TestSaveRoleHashIsolatesUids(t *testing.T) {
	client := withHashStub(t)
	a, b := NewRole(710008), NewRole(710009)
	a.PbRole.BasicInfo.Name = "role_a"
	b.PbRole.BasicInfo.Name = "role_b"

	if err := saveRoleHash(context.Background(), a, true); err != nil {
		t.Fatal(err)
	}
	if err := saveRoleHash(context.Background(), b, true); err != nil {
		t.Fatal(err)
	}

	if len(client.data[roleHashKey(710008)]) == 0 || len(client.data[roleHashKey(710009)]) == 0 {
		t.Fatalf("两个 uid 的 hash 均应有数据")
	}
	ia, _ := loadRoleHash(710008)
	ib, _ := loadRoleHash(710009)
	if ia.GetBasicInfo().GetName() != "role_a" || ib.GetBasicInfo().GetName() != "role_b" {
		t.Fatalf("数据串扰: a=%q b=%q", ia.GetBasicInfo().GetName(), ib.GetBasicInfo().GetName())
	}
}

// HSetFieldsExpire 实例缺失错误路径（Redis 侧异常不静默）。
func TestHSetFieldsExpireMissingInstance(t *testing.T) {
	old := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	t.Cleanup(func() { rds.RedisMgr = old })

	if err := rds.RedisMgr.HSetFieldsExpire(context.Background(), 99, "k",
		map[string][]byte{"f": []byte("v")}, time.Minute); err == nil {
		t.Fatal("实例不存在应返回 error")
	}
}

// 编译期契约：retryL3 满足 dal.L3Store。
var _ dal.L3Store = (*retryL3)(nil)
