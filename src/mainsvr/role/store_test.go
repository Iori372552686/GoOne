package role

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/db/dal"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	goredis "github.com/redis/go-redis/v9"
)

// ============ 桩：Redis L2 ============

// hashRedisClient 只实现角色 L2 用到的命令（HGetAll/HSet/Expire/Del）。
type hashRedisClient struct {
	goredis.UniversalClient
	data map[string]map[string]string
}

func (c *hashRedisClient) HGetAll(_ context.Context, key string) *goredis.MapStringStringCmd {
	cmd := goredis.NewMapStringStringCmd(context.Background())
	fields := map[string]string{}
	for f, v := range c.data[key] {
		fields[f] = v
	}
	cmd.SetVal(fields)
	return cmd
}

func (c *hashRedisClient) HSet(_ context.Context, key string, values ...interface{}) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(context.Background())
	if m, ok := values[0].(map[string]interface{}); ok {
		if c.data[key] == nil {
			c.data[key] = map[string]string{}
		}
		for f, v := range m {
			c.data[key][f] = string(v.([]byte))
		}
	}
	cmd.SetVal(1)
	return cmd
}

func (c *hashRedisClient) Expire(context.Context, string, time.Duration) *goredis.BoolCmd {
	return goredis.NewBoolResult(true, nil)
}

func (c *hashRedisClient) Del(_ context.Context, keys ...string) *goredis.IntCmd {
	for _, k := range keys {
		delete(c.data, k)
	}
	return goredis.NewIntResult(1, nil)
}

func withHashRedis(t *testing.T) *hashRedisClient {
	t.Helper()
	client := &hashRedisClient{data: map[string]map[string]string{}}
	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	rds.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_ROLE), client)
	t.Cleanup(func() { rds.RedisMgr = oldMgr })
	return client
}

// ============ 桩：L3 ============

type fakeRoleL3 struct {
	data      map[uint64][]byte
	loaded    int
	saveCalls int
	lastSaved []byte
	lastTime  int64
	loadErr   error
}

func (f *fakeRoleL3) Load(_ context.Context, uid uint64) ([]byte, int64, error) {
	f.loaded++
	if f.loadErr != nil {
		return nil, 0, f.loadErr
	}
	d, ok := f.data[uid]
	if !ok {
		return nil, 0, nil
	}
	return d, 1, nil
}

func (f *fakeRoleL3) Save(_ context.Context, uid uint64, data []byte, ts int64) error {
	f.saveCalls++
	if f.data == nil {
		f.data = map[uint64][]byte{}
	}
	f.data[uid] = data
	f.lastSaved = data
	f.lastTime = ts
	return nil
}

func (f *fakeRoleL3) Delete(_ context.Context, _ uint64) error { return nil }

func newTestStore(t *testing.T) (*RoleStore, *hashRedisClient, *fakeRoleL3) {
	t.Helper()
	client := withHashRedis(t)
	l3 := &fakeRoleL3{}
	store := NewRoleStore(func() int64 { return 7777 })
	store.setL3(l3)
	return store, client, l3
}

// ============ 测试 ============

// L3 权威性端到端：写角色 → 清空 L2（模拟 TTL 过期/丢 key）→ Load 从 L3 恢复
// 并回填 L2。这是"TTL 可以安全开启"的核心验收。
func TestRoleStoreLoadRecoversFromL3WhenL2Lost(t *testing.T) {
	store, client, l3 := newTestStore(t)
	uid := uint64(880001)

	r := NewRole(uid)
	r.PbRole.BasicInfo.Name = "l3_survivor"
	r.PbRole.BasicInfo.Exp = 424242
	r.PbRole.InventoryInfo.ItemMap[2001] = &g1_protocol.PbItem{Id: 2001, Count: 9}

	// 1) 全量写 L2，再手动投递 L3 快照（等价运行期双写）。
	if err := saveRoleHash(context.Background(), r, true); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveL3(context.Background(), uid, r.PbRole); err != nil {
		t.Fatal(err)
	}

	// 2) 模拟 L2 全丢（TTL 过期 / Redis 清库）。
	client.data = map[string]map[string]string{}

	// 3) 读穿：必须从 L3 恢复且回填 L2。
	info, found, err := store.Load(context.Background(), nil, uid)
	if err != nil || !found {
		t.Fatalf("Load after L2 loss = (found=%v, err=%v)，L3 必须兜底", found, err)
	}
	if info.BasicInfo.Name != "l3_survivor" || info.BasicInfo.Exp != 424242 {
		t.Fatalf("L3 恢复数据错误: name=%q exp=%d", info.BasicInfo.Name, info.BasicInfo.Exp)
	}
	if got := info.InventoryInfo.ItemMap[2001].GetCount(); got != 9 {
		t.Fatalf("inventory 恢复错误: count=%d", got)
	}
	if l3.loaded == 0 {
		t.Fatal("未回源 L3")
	}
	key := roleHashKey(uid)
	if len(client.data[key]) == 0 {
		t.Fatal("L3 命中后未回填 L2")
	}
}

// 双 miss = 新角色。
func TestRoleStoreLoadDoubleMissIsNewRole(t *testing.T) {
	store, _, l3 := newTestStore(t)
	l3.data = map[uint64][]byte{}

	_, found, err := store.Load(context.Background(), nil, uint64(880002))
	if err != nil || found {
		t.Fatalf("double miss = (found=%v, err=%v), want (false,nil)", found, err)
	}
}

// L3 不可用时必须 fail-closed：绝不能把"读不到 L3"当成"没有存量数据"。
func TestRoleStoreLoadFailsClosedWhenL3Down(t *testing.T) {
	store, _, l3 := newTestStore(t)
	l3.loadErr = errors.New("mysqlsvr unreachable")

	if _, _, err := store.Load(context.Background(), nil, uint64(880003)); err == nil {
		t.Fatal("L3 故障时 Load 必须报错，不得静默当作新角色（防数据覆盖）")
	}
}

// L2 命中不触达 L3（热路径零额外开销）。
func TestRoleStoreLoadL2HitSkipsL3(t *testing.T) {
	store, _, l3 := newTestStore(t)
	uid := uint64(880004)

	r := NewRole(uid)
	if err := saveRoleHash(context.Background(), r, true); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.Load(context.Background(), nil, uid); err != nil || !found {
		t.Fatalf("L2 hit load = (%v,%v)", found, err)
	}
	if l3.loaded != 0 {
		t.Fatal("L2 命中不应触达 L3")
	}
}

// codec 契约：L3 整包序列化必须清 ConnSvrInfo 且往返无损（含无 flag 的 GiftInfo）。
func TestRoleCodecL3RoundTripClearsConnSvrInfo(t *testing.T) {
	r := NewRole(uint64(880005))
	r.PbRole.ConnSvrInfo.BusId = 99
	r.PbRole.ConnSvrInfo.ClientPos = "1.2.3.4:5"
	r.PbRole.BasicInfo.Name = "codec_check"
	r.PbRole.GiftInfo = &g1_protocol.RoleGiftExchangeInfo{TypeMap: map[int32]int32{3: 1}}

	var codec roleCodec
	buf, err := codec.MarshalL3(r.PbRole)
	if err != nil {
		t.Fatal(err)
	}
	// 序列化不得修改内存中的 Role（clone 隔离）。
	if r.PbRole.ConnSvrInfo == nil || r.PbRole.ConnSvrInfo.BusId != 99 {
		t.Fatal("MarshalL3 污染了内存中的 ConnSvrInfo")
	}

	info, err := codec.UnmarshalL3(buf)
	if err != nil {
		t.Fatal(err)
	}
	if info.ConnSvrInfo != nil {
		t.Fatal("L3 快照不应包含 ConnSvrInfo（运行时状态）")
	}
	if info.BasicInfo.Name != "codec_check" {
		t.Fatalf("name = %q", info.BasicInfo.Name)
	}
	if info.GiftInfo == nil || info.GiftInfo.TypeMap[3] != 1 {
		t.Fatal("GiftInfo（无 section flag）在整包快照中丢失")
	}
}

// codec 契约：全量 L2 字段集合与 roleSectionAccessors + gift 对齐（与 saveRoleHash 同源）。
func TestRoleCodecL2FieldsMatchAccessors(t *testing.T) {
	r := NewRole(uint64(880006))
	r.PbRole.GiftInfo = &g1_protocol.RoleGiftExchangeInfo{TypeMap: map[int32]int32{1: 2}}

	var codec roleCodec
	fields, err := codec.MarshalL2Fields(r.PbRole, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(fields))
	for f := range fields {
		names = append(names, f)
	}
	sort.Strings(names)
	if len(names) != len(roleSectionAccessors)+1 {
		t.Fatalf("全量 L2 字段数 = %d, want %d（accessors+gift）", len(names), len(roleSectionAccessors)+1)
	}

	// 子集选择。
	sub, err := codec.MarshalL2Fields(r.PbRole, []string{"basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) != 1 || sub["basic"] == nil {
		t.Fatalf("子集写入错误: %v", sub)
	}
}

// saveRoleHash 成功后置 needL3Flush；MaybeFlushL3 在防抖窗口内不投递、force 立即投递。
func TestSaveRoleHashMarksNeedL3Flush(t *testing.T) {
	withHashRedis(t)
	r := NewRole(uint64(880007))
	if r.needL3Flush {
		t.Fatal("新角色不应有待写标记")
	}
	if err := saveRoleHash(context.Background(), r, true); err != nil {
		t.Fatal(err)
	}
	if !r.needL3Flush {
		t.Fatal("L2 成功后必须置 needL3Flush")
	}
	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatalf("防抖窗口内的 MaybeFlushL3 不应投递: %v", err)
	}
}

// MaybeFlushL3 force 投递经注入的 roleStore L3 桩验证。
func TestMaybeFlushL3ForceSendsSnapshot(t *testing.T) {
	client := withHashRedis(t)
	l3 := &fakeRoleL3{}
	old := roleStore
	roleStore = NewRoleStore(func() int64 { return 12345 })
	roleStore.setL3(l3)
	t.Cleanup(func() { roleStore = old })

	r := NewRole(uint64(880008))
	if err := saveRoleHash(context.Background(), r, true); err != nil {
		t.Fatal(err)
	}
	_ = client
	if err := r.MaybeFlushL3(true); err != nil {
		t.Fatalf("force flush: %v", err)
	}
	if l3.saveCalls != 1 {
		t.Fatalf("L3 投递次数 = %d, want 1", l3.saveCalls)
	}
	if l3.lastTime != 12345 {
		t.Fatalf("updateTime = %d, want 12345（服务器时钟）", l3.lastTime)
	}
	if r.needL3Flush {
		t.Fatal("投递成功后标记应清除")
	}
}

// 编译期接口断言。
var (
	_ dal.L2Cache                            = redisRoleL2{}
	_ dal.L3Store                            = (*mysqlRoleL3)(nil)
	_ dal.EntityCodec[*g1_protocol.RoleInfo] = roleCodec{}
)
