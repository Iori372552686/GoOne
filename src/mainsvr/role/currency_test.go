package role

import (
	"context"
	"math"
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// fakeRoleL3 L3 桩：只统计 Save 投递次数，Load/Delete 供接口完备。
type fakeRoleL3 struct {
	saveCalls int
}

func (f *fakeRoleL3) Load(ctx context.Context, key uint64) (data []byte, updateTime int64, err error) {
	return nil, 0, nil
}

func (f *fakeRoleL3) Save(ctx context.Context, key uint64, data []byte, updateTime int64) error {
	f.saveCalls++
	return nil
}

func (f *fakeRoleL3) Delete(ctx context.Context, key uint64) error { return nil }

func newCurrencyTestRole(t *testing.T, uid uint64) *Role {
	t.Helper()
	r := NewRole(uid)
	r.PbRole.ConnSvrInfo.BusId = 1
	return r
}

// 基础增减查：Add/Deduct/Get/CheckEnough。
func TestCurrencyAddDeductGet(t *testing.T) {
	r := newCurrencyTestRole(t, 5001)
	c := r.Currency
	gold := int32(g1_protocol.EItemID_GOLD)

	// 初始默认 1000000 + OnRoleCreate 首发礼包 500000
	if got := c.Get(gold); got != 1500000 {
		t.Fatalf("initial gold = %d, want 1500000", got)
	}
	if ret := c.Add(gold, 500, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("add: %v", ret)
	}
	if got := c.Get(gold); got != 1500500 {
		t.Fatalf("after add = %d, want 1500500", got)
	}
	if !c.CheckEnough(gold, 1500500) || c.CheckEnough(gold, 1500501) {
		t.Fatal("CheckEnough 语义错误")
	}
	if ret := c.Deduct(gold, 500000, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("deduct: %v", ret)
	}
	if got := c.Get(gold); got != 1000500 {
		t.Fatalf("after deduct = %d, want 1000500", got)
	}
}

// 边界：零静默成功、负数拒绝、余额不足原子不扣。
func TestCurrencyBoundaries(t *testing.T) {
	r := newCurrencyTestRole(t, 5002)
	c := r.Currency
	gold := int32(g1_protocol.EItemID_GOLD)

	if ret := c.Add(gold, 0, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("zero add should be silent ok, got %v", ret)
	}
	if ret := c.Deduct(gold, 0, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("zero deduct should be silent ok, got %v", ret)
	}
	if ret := c.Add(gold, -1, nil); ret != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("negative add = %v, want ERR_ARGV", ret)
	}
	if ret := c.Deduct(gold, -1, nil); ret != g1_protocol.ErrorCode_ERR_ARGV {
		t.Fatalf("negative deduct = %v, want ERR_ARGV", ret)
	}

	before := c.Get(gold)
	if ret := c.Deduct(gold, before+1, nil); ret != g1_protocol.ErrorCode_ERR_ITEM_NOT_ENOUGH {
		t.Fatalf("insufficient = %v, want ERR_ITEM_NOT_ENOUGH", ret)
	}
	if got := c.Get(gold); got != before {
		t.Fatalf("insufficient deduct must not partially apply: %d -> %d", before, got)
	}
}

// 溢出保护：加到 MaxInt64 再加必须拒绝且余额不变。
func TestCurrencyOverflow(t *testing.T) {
	r := newCurrencyTestRole(t, 5003)
	c := r.Currency
	medal := int32(g1_protocol.EItemID_LIVENESS)

	if ret := c.Add(medal, math.MaxInt64, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("seed max: %v", ret)
	}
	if got := c.Get(medal); got != math.MaxInt64 {
		t.Fatalf("seed = %d, want MaxInt64", got)
	}
	if ret := c.Add(medal, 1, nil); ret != g1_protocol.ErrorCode_ERR_ITEM_ADD_ERROR {
		t.Fatalf("overflow add = %v, want ERR_ITEM_ADD_ERROR", ret)
	}
	if got := c.Get(medal); got != math.MaxInt64 {
		t.Fatalf("overflow must not apply: %d", got)
	}
}

// 段判定边界：1 与 9999 是货币，0 与 10000 不是。
func TestCurrencyIsCurrencyBoundaries(t *testing.T) {
	r := newCurrencyTestRole(t, 5004)
	c := r.Currency

	if !c.IsCurrency(1) || !c.IsCurrency(9999) {
		t.Fatal("1 与 9999 应为货币段")
	}
	if c.IsCurrency(0) || c.IsCurrency(10000) || c.IsCurrency(-5) {
		t.Fatal("0 / 10000 / 负数 不应为货币段")
	}
	if !isCurrencyID(2) || isCurrencyID(10001) {
		t.Fatal("包级判定与组件判定应一致")
	}
}

// InitField 默认值：空 map 注入初始资源；已有数据不覆盖。
func TestCurrencyInitFieldDefaults(t *testing.T) {
	r := newCurrencyTestRole(t, 5005)

	if got := r.Currency.Get(int32(g1_protocol.EItemID_DIAMOND)); got != 510000 {
		t.Fatalf("diamond = %d, want 510000 (10000 默认 + 500000 首发礼包)", got)
	}
	if got := r.Currency.Get(int32(g1_protocol.EItemID_CREDIT)); got != 510000 {
		t.Fatalf("credit = %d, want 510000 (10000 默认 + 500000 首发礼包)", got)
	}
	if got := r.Currency.Get(int32(g1_protocol.EItemID_STAMINA)); got != 0 {
		t.Fatalf("stamina = %d, want 0（未配置默认值的货币不注入）", got)
	}

	// 已有数据不重置
	r.PbRole.CurrencyInfo.CurrencyMap[int32(g1_protocol.EItemID_GOLD)] = 7
	r.Currency.InitField(5005)
	if got := r.Currency.Get(int32(g1_protocol.EItemID_GOLD)); got != 7 {
		t.Fatalf("InitField 不应覆盖既有数据, got %d", got)
	}
}

// 增量同步：只推变更币种；ClearDirty 后不再产生 patch。
func TestCurrencyPatchOnlyChanged(t *testing.T) {
	r := newCurrencyTestRole(t, 5006)
	c := r.Currency

	// 清掉 NewRole/OnInit 期间的变更痕迹
	r.Currency.ClearDirty()
	v2 := &g1_protocol.ScSyncUserDataV2{}
	if c.BuildPatch(v2) {
		t.Fatal("无变更不应产出 patch")
	}

	_ = c.Add(int32(g1_protocol.EItemID_GOLD), 1, nil)
	_ = c.Deduct(int32(g1_protocol.EItemID_DIAMOND), 1, nil)

	v2 = &g1_protocol.ScSyncUserDataV2{}
	if !c.BuildPatch(v2) || v2.CurrencyPatch == nil {
		t.Fatal("有变更应产出 currency patch")
	}
	if len(v2.CurrencyPatch.Changed) != 2 {
		t.Fatalf("changed = %d, want 2", len(v2.CurrencyPatch.Changed))
	}
	if v2.CurrencyPatch.Changed[int32(g1_protocol.EItemID_GOLD)] != 1500001 {
		t.Fatalf("gold in patch = %d, want 1500001", v2.CurrencyPatch.Changed[int32(g1_protocol.EItemID_GOLD)])
	}
	if v2.CurrencyPatch.Changed[int32(g1_protocol.EItemID_DIAMOND)] != 509999 {
		t.Fatalf("diamond in patch = %d, want 509999", v2.CurrencyPatch.Changed[int32(g1_protocol.EItemID_DIAMOND)])
	}

	c.ClearDirty()
	if c.BuildPatch(&g1_protocol.ScSyncUserDataV2{}) {
		t.Fatal("ClearDirty 后不应再产出 patch")
	}
}

// L3Critical：货币变更后无视防抖立即投递；无变更时受防抖约束。
func TestCurrencyCriticalBypassesL3Debounce(t *testing.T) {
	// Redis 桩：MaybeFlushL3 的链路不触 Redis（仅 L3 投递），但 saveRoleHash 需要；
	// 本用例直接置 needL3Flush，无需 Redis。
	l3 := &fakeRoleL3{}
	old := roleStore
	roleStore = NewRoleStore(func() int64 { return 777 })
	roleStore.setL3(l3)
	t.Cleanup(func() { roleStore = old })

	r := newCurrencyTestRole(t, 5007)
	r.needL3Flush = true
	// 无货币变更（l3Pending=false）+ lastL3FlushAt==0：受防抖拦截
	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatalf("debounced no-op should be nil: %v", err)
	}
	if l3.saveCalls != 0 {
		t.Fatalf("防抖窗口内不应投递, calls=%d", l3.saveCalls)
	}

	// 货币变更 → l3Pending → 无视防抖立即投递
	_ = r.Currency.Add(int32(g1_protocol.EItemID_GOLD), 1, nil)
	if !r.Currency.RequireImmediateL3() {
		t.Fatal("货币变更后 RequireImmediateL3 应为 true")
	}
	if err := r.MaybeFlushL3(false); err != nil {
		t.Fatalf("critical flush: %v", err)
	}
	if l3.saveCalls != 1 {
		t.Fatalf("critical 应立即投递, calls=%d", l3.saveCalls)
	}
	if r.Currency.RequireImmediateL3() {
		t.Fatal("投递成功后 l3Pending 应清除")
	}
}

// 变更回调：OnChange 注册的钩子在增减时被调用且值正确。
func TestCurrencyOnChangeHook(t *testing.T) {
	r := newCurrencyTestRole(t, 5008)
	c := r.Currency

	type rec struct {
		id          int32
		before, aft int64
	}
	got := make([]rec, 0, 2)
	c.OnChange(func(id int32, before, after int64) {
		got = append(got, rec{id, before, after})
	})

	_ = c.Add(int32(g1_protocol.EItemID_LIVENESS), 10, nil)
	_ = c.Deduct(int32(g1_protocol.EItemID_LIVENESS), 4, nil)

	if len(got) != 2 {
		t.Fatalf("hooks = %d, want 2", len(got))
	}
	if got[0].before != 0 || got[0].aft != 10 {
		t.Fatalf("add hook = %+v", got[0])
	}
	if got[1].before != 10 || got[1].aft != 6 {
		t.Fatalf("deduct hook = %+v", got[1])
	}
}

// ItemAdd/ItemReduce 对货币 ID 的路由：经统一道具入口仍落到 currency_map。
func TestCurrencyRoutingViaItemAPI(t *testing.T) {
	r := newCurrencyTestRole(t, 5009)
	gold := int32(g1_protocol.EItemID_GOLD)

	if ret := r.ItemAdd(gold, 100, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("ItemAdd currency: %v", ret)
	}
	if got := r.GetItemCount(gold); got != 1500100 {
		t.Fatalf("after ItemAdd = %d, want 1500100", got)
	}
	if _, ret := r.ItemReduce(gold, 100, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("ItemReduce currency: %v", ret)
	}
	if got := r.GetItemCount(gold); got != 1500000 {
		t.Fatalf("after ItemReduce = %d, want 1500000", got)
	}

	// 经验（EXP）也是货币：GetBriefInfo 从货币读取
	if ret := r.ItemAdd(int32(g1_protocol.EItemID_EXP), 123, nil); ret != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("ItemAdd exp: %v", ret)
	}
	if got := r.Currency.Get(int32(g1_protocol.EItemID_EXP)); got != 123 {
		t.Fatalf("exp = %d, want 123", got)
	}
	brief := r.GetBriefInfo()
	if brief.Exp != 123 {
		t.Fatalf("brief exp = %d, want 123", brief.Exp)
	}
}
