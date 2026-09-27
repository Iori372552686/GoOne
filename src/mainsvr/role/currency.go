package role

import (
	"math"

	pb "github.com/Iori372552686/g1_common/protocol"
)

// ============================================================
// 货币组件：全部资源型数值（金币/钻石/绑定钻石/经验/体力/各类代币）的
// 唯一权威存储，挂在 RoleInfo.currency_info（map<货币ID,余额>）。
//
// 货币 ID 段：1-9999（EItemID 货币段）；10000+ 为背包道具，归 ItemComponent。
// 同步走增量档（只推变更币种）；货币为高频写段，标记 L3Critical——
// 变更后无视 60s 防抖立即投递 L3 快照，缓解 L2 唯一副本风险。
// ============================================================

// CurrencyComponent 货币组件。
type CurrencyComponent struct {
	sectionBase

	// changed 变更币种集合（客户端增量同步用；货币无删除语义）。
	changed int32Set
	// l3Pending 自上次 L3 快照后是否有货币变更（L3Critical 判定）。
	l3Pending bool
	// hooks 变更回调（任务进度/统计等扩展点，UID 串行域内同步调用）。
	hooks []func(id int32, before, after int64)
}

func NewCurrencyComponent() *CurrencyComponent { return &CurrencyComponent{} }

func (c *CurrencyComponent) Name() string { return "currency" }

func (c *CurrencyComponent) Flag() pb.ERoleSectionFlag {
	return pb.ERoleSectionFlag_CURRENCY_INFO
}

func (c *CurrencyComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *CurrencyComponent) OnDestroy()           {}

// InitField 货币段兜底与初始资源（原 RoleBasicInfo 标量初值迁移至此）。
func (c *CurrencyComponent) InitField(uid uint64) {
	info := c.role.PbRole.CurrencyInfo
	if info == nil {
		info = &pb.RoleCurrencyInfo{CurrencyMap: make(map[int32]int64)}
		c.role.PbRole.CurrencyInfo = info
	}
	if info.CurrencyMap == nil {
		info.CurrencyMap = make(map[int32]int64)
	}
	if len(info.CurrencyMap) == 0 {
		info.CurrencyMap[int32(pb.EItemID_GOLD)] = 1000000
		info.CurrencyMap[int32(pb.EItemID_DIAMOND)] = 10000
		info.CurrencyMap[int32(pb.EItemID_CREDIT)] = 10000
	}
}

func (c *CurrencyComponent) Touch(reason string) {
	c.touch(pb.ERoleSectionFlag_CURRENCY_INFO, reason)
}

// MarkDirty 增量档脏标记；货币无删除语义，deleted 忽略。
func (c *CurrencyComponent) MarkDirty(id int32, deleted bool) {
	c.role.markPatchSection(pb.ERoleSectionFlag_CURRENCY_INFO)
	setInt32Value(&c.changed, id)
	c.role.markPersistSectionDirty(pb.ERoleSectionFlag_CURRENCY_INFO, "currency")
}

func (c *CurrencyComponent) BuildPatch(dst *pb.ScSyncUserDataV2) bool {
	if len(c.changed) == 0 {
		return false
	}
	patch := &pb.RoleCurrencyPatch{Changed: make(map[int32]int64, len(c.changed))}
	info := c.role.PbRole.CurrencyInfo
	for id := range c.changed {
		if info != nil {
			patch.Changed[id] = info.CurrencyMap[id]
		} else {
			patch.Changed[id] = 0
		}
	}
	dst.CurrencyPatch = patch
	return true
}

func (c *CurrencyComponent) ClearDirty() {
	c.changed = nil
}

// RequireImmediateL3 货币变更后待 L3 快照 → 无视防抖立即投递。
func (c *CurrencyComponent) RequireImmediateL3() bool { return c.l3Pending }

// OnL3Flushed L3 快照投递成功后清除待写标记。
func (c *CurrencyComponent) OnL3Flushed() { c.l3Pending = false }

// ===== 查询 =====

// IsCurrency 货币段判定（1-9999）。纯段判定，无配置依赖。
func (c *CurrencyComponent) IsCurrency(id int32) bool {
	return isCurrencyID(id)
}

// isCurrencyID 包级货币段判定（无 Role 上下文的纯函数场景，如获得展示聚合）。
func isCurrencyID(id int32) bool {
	return id >= 1 && id < currencyIDMax
}

// currencyIDMax 货币段上界（不含）：1-9999 货币，10000 起为背包道具。
const currencyIDMax int32 = 10000

// Get 取货币余额；未持有返回 0。
func (c *CurrencyComponent) Get(id int32) int64 {
	info := c.role.PbRole.CurrencyInfo
	if info == nil {
		return 0
	}
	return info.CurrencyMap[id]
}

// CheckEnough 余额是否足够。
func (c *CurrencyComponent) CheckEnough(id int32, amount int64) bool {
	return c.Get(id) >= amount
}

// Currencies 返回货币表只读视图（登录全量同步/调试用）。
func (c *CurrencyComponent) Currencies() map[int32]int64 {
	info := c.role.PbRole.CurrencyInfo
	if info == nil {
		return nil
	}
	return info.CurrencyMap
}

// OnChange 注册变更回调；回调在 UID 串行域内同步执行。
func (c *CurrencyComponent) OnChange(cb func(id int32, before, after int64)) {
	if cb != nil {
		c.hooks = append(c.hooks, cb)
	}
}

// ===== 变更 =====

// Add 增加货币。负数拒绝、零静默成功；余额溢出保护。
func (c *CurrencyComponent) Add(id int32, amount int64, reason *Reason) pb.ErrorCode {
	if amount < 0 {
		return pb.ErrorCode_ERR_ARGV
	}
	if amount == 0 {
		return pb.ErrorCode_ERR_OK
	}
	info := c.ensureInfo()
	before := info.CurrencyMap[id]
	if before > math.MaxInt64-amount {
		c.role.Errorf("CURRENCY|overflow {id:%d, have:%d, add:%d}", id, before, amount)
		return pb.ErrorCode_ERR_ITEM_ADD_ERROR
	}
	after := before + amount
	info.CurrencyMap[id] = after
	c.afterChange(id, before, after, reason)
	return pb.ErrorCode_ERR_OK
}

// Deduct 扣减货币。负数拒绝、零静默成功；余额不足拒绝（原子性：不足时不产生部分扣减）。
func (c *CurrencyComponent) Deduct(id int32, amount int64, reason *Reason) pb.ErrorCode {
	if amount < 0 {
		return pb.ErrorCode_ERR_ARGV
	}
	if amount == 0 {
		return pb.ErrorCode_ERR_OK
	}
	info := c.ensureInfo()
	before := info.CurrencyMap[id]
	if before < amount {
		return pb.ErrorCode_ERR_ITEM_NOT_ENOUGH
	}
	after := before - amount
	info.CurrencyMap[id] = after
	c.afterChange(id, before, after, reason)
	return pb.ErrorCode_ERR_OK
}

// ensureInfo 懒初始化货币段（老数据/异常路径兜底）。
func (c *CurrencyComponent) ensureInfo() *pb.RoleCurrencyInfo {
	info := c.role.PbRole.CurrencyInfo
	if info == nil {
		info = &pb.RoleCurrencyInfo{}
		c.role.PbRole.CurrencyInfo = info
	}
	if info.CurrencyMap == nil {
		info.CurrencyMap = make(map[int32]int64)
	}
	return info
}

// afterChange 变更后置：脏标记（客户端增量 + 持久化）+ L3 待写 + 回调 + 日志。
// 与既有道具域一致：INIT 期变更（角色创建首发）不标脏——创建路径本身
// 走 force 全量落盘与 L3 强刷，数据已覆盖。
func (c *CurrencyComponent) afterChange(id int32, before, after int64, reason *Reason) {
	if shouldTrackMutation(reason) {
		c.MarkDirty(id, false)
		c.l3Pending = true
	}
	for _, h := range c.hooks {
		h(id, before, after)
	}
	if reason != nil {
		c.role.Debugf("CURRENCY|change {id:%d, %d -> %d, reason:[%v|%v]}", id, before, after,
			reason.Reason, reason.Scene)
	} else {
		c.role.Debugf("CURRENCY|change {id:%d, %d -> %d, reason:nil}", id, before, after)
	}
}

// currencySection 货币段静态描述。
func currencySection() roleSection {
	return messageSection(pb.ERoleSectionFlag_CURRENCY_INFO, "currency",
		func(i *pb.RoleInfo) *pb.RoleCurrencyInfo { return i.CurrencyInfo },
		func(i *pb.RoleInfo, m *pb.RoleCurrencyInfo) { i.CurrencyInfo = m },
		func() *pb.RoleCurrencyInfo { return new(pb.RoleCurrencyInfo) })
}
