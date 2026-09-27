/// 道具相关

package role

import (
	itemconf "github.com/Iori372552686/GoOne/module/gamedata/repository/item"
	pb "github.com/Iori372552686/g1_common/protocol"
)

/// 玩家道具/货币相关操作放在这里。
// 货币（1-9999）统一走 CurrencyComponent（RoleCurrencyInfo），
// 背包道具（10000+）走 InventoryInfo——三处旧平行映射已收敛为段判定。

// 获取道具/货币数量。
func (r *Role) GetItemCount(itemId int32) int64 {
	if r.Currency.IsCurrency(itemId) {
		return r.Currency.Get(itemId)
	}
	if it := r.PbRole.InventoryInfo.ItemMap[itemId]; it != nil {
		return it.Count
	}
	return 0
}

func (r *Role) ItemCheckAdd(itemId int32, itemCount int64) int {
	if itemCount <= 0 {
		return int(pb.ErrorCode_ERR_ARGV)
	}
	// 货币无拥有上限语义（入 currency_map，不占背包）
	if r.Currency.IsCurrency(itemId) {
		return 0
	}
	itemConf := itemconf.GetItemByItemId(itemId)
	if itemConf == nil {
		return int(pb.ErrorCode_ERR_CONF)
	}
	// 普通道具检查 MaxOwnCount 上限
	rule := r.getItemRule(itemConf)
	if rule != nil && rule.MaxOwnCount > 0 {
		cur := r.GetItemCount(itemId)
		if cur+itemCount > int64(rule.MaxOwnCount) {
			r.Errorf("ITEM|over limit {id:%d, cur:%d, add:%d, max:%d}",
				itemId, cur, itemCount, rule.MaxOwnCount)
			return int(pb.ErrorCode_ERR_ITEM_OVER_LIMIT)
		}
	}
	return 0
}

// getItemRule 取道具规则配置。
// ItemConfig 当前未携带 RuleID 字段，通过独立的 ItemRuleRefConfig(ItemId→RuleId) 间接查得，
// 再到 ItemRuleConfig 取规则细节。任一环节缺失返回 nil。
func (r *Role) getItemRule(itemConf *pb.ItemConfig) *pb.ItemRuleConfig {
	if itemConf == nil {
		return nil
	}
	ref := itemconf.GetItemRuleRefByItemId(itemConf.ItemId)
	if ref == nil || ref.RuleId == 0 {
		return nil
	}
	return itemconf.GetItemRuleById(ref.RuleId)
}

func (r *Role) ItemsCheckAdd(items *[]*pb.PbItem) pb.ErrorCode {
	for _, v := range *items {
		if 0 != r.ItemCheckAdd(v.Id, v.Count) {
			return pb.ErrorCode_ERR_ITEM_ADD_ERROR
		}
	}
	return pb.ErrorCode_ERR_OK
}

// 生成掉落（如果输入列表里面有drop类型的道具，则展开drop）
func (r *Role) ItemsSee(in *[]*pb.PbItem) *[]*pb.PbItem {
	out := make([]*pb.PbItem, 0)

	for _, v := range *in {
		itemOut := r.ItemSee(v)
		out = append(out, *itemOut...)
	}
	return &out
}

// ItemSee 输入道具的"视图展开"：
//   - 货币直通（不依赖 ItemConfig）；
//   - 未配置道具丢弃（与历史行为一致，防幽灵道具入包）；
//   - 其余原样返回。掉落产出统一走 DropExecute（drop_group.go 双层解析），
//     不再在此做单层展开。
func (r *Role) ItemSee(item *pb.PbItem) *[]*pb.PbItem {
	out := make([]*pb.PbItem, 0)
	if r.Currency.IsCurrency(item.Id) {
		out = append(out, item)
		return &out
	}
	if itemconf.GetItemByItemId(item.Id) == nil {
		return &out
	}
	out = append(out, item)
	return &out
}

// 返回道具消耗后获得的新道具，比如开宝箱消耗一个宝箱，获得宝箱内的道具
func (r *Role) ItemCheckReduce(itemId int32, itemCount int64) (*[]*pb.PbItem, pb.ErrorCode) {
	ret := pb.ErrorCode_ERR_OK
	if itemCount == 0 {
		return nil, ret
	}

	have := r.GetItemCount(itemId)
	outcomes := r.GetItemOutcomes(itemId)
	if have < itemCount {
		switch itemId {
		case int32(pb.EItemID_GOLD):
			ret = pb.ErrorCode_ERR_GOLD_NOT_ENOUGH
		case int32(pb.EItemID_DIAMOND):
			ret = pb.ErrorCode_ERR_DIAMOND_NOT_ENOUGH
		case int32(pb.EItemID_CREDIT):
			ret = pb.ErrorCode_ERR_SINEW_NOT_ENOUGH
		default:
			ret = pb.ErrorCode_ERR_ITEM_NOT_ENOUGH // 默认值
		}
	}
	return outcomes, ret
}

func (r *Role) ItemsCheckReduce(items *[]*pb.PbItem) (*[]*pb.PbItem, pb.ErrorCode) {
	items = r.itemAggregate(items)
	outcomes := make([]*pb.PbItem, 0)
	for _, v := range *items {
		out, ret := r.ItemCheckReduce(v.Id, v.Count)
		if ret != 0 {
			return nil, ret
		} else {
			if out != nil {
				outcomes = append(outcomes, *out...)
			}
		}
	}

	if ret := r.ItemsCheckAdd(&outcomes); ret != 0 {
		return nil, ret
	}

	return &outcomes, pb.ErrorCode_ERR_OK
}

// 反回实际添加的东西
func (r *Role) ItemReduce(itemId int32, itemCount int64, reason *Reason) (*[]*pb.PbItem, pb.ErrorCode) {
	if itemCount == 0 {
		return nil, pb.ErrorCode_ERR_OK
	}

	out, ret := r.ItemCheckReduce(itemId, itemCount)
	if pb.ErrorCode_ERR_OK != ret {
		return nil, ret
	}

	if r.Currency.IsCurrency(itemId) {
		// 货币扣减（含余额校验与脏标记，CurrencyComponent 内完成）
		if ret := r.Currency.Deduct(itemId, itemCount, reason); ret != pb.ErrorCode_ERR_OK {
			return nil, ret
		}
		return out, pb.ErrorCode_ERR_OK
	}

	it := r.PbRole.InventoryInfo.ItemMap[itemId]
	if it == nil {
		r.Errorf("reduce nil item {id: %v}", itemId)
		return nil, pb.ErrorCode_ERR_ITEM_NOT_ENOUGH
	}
	it.Count -= itemCount
	if it.Count == 0 {
		r.ItemRemove(itemId)
	} else {
		r.trackItemMutation(itemId, false, reason)
	}

	r.Debugf("reduce item {id: %v, cnt: %v, after: %v, reason:[%d|%d]}",
		itemId, itemCount, it.Count, reason.Reason, reason.Scene)

	return out, pb.ErrorCode_ERR_OK
}

func (r *Role) ItemsReduce(items *[]*pb.PbItem, reason *Reason) (*[]*pb.PbItem, pb.ErrorCode) {
	if len(*items) == 0 {
		return nil, 0
	}

	out, ret := r.ItemsCheckReduce(items)
	if 0 != ret {
		return nil, ret
	}

	for _, v := range *items {
		if v.Count == 0 {
			continue
		}
		if r.Currency.IsCurrency(v.Id) {
			if ret := r.Currency.Deduct(v.Id, v.Count, reason); ret != pb.ErrorCode_ERR_OK {
				return nil, ret
			}
			continue
		}
		it := r.PbRole.InventoryInfo.ItemMap[v.Id]
		if it == nil {
			r.Errorf("reduce nil item {id: %v}", v.Id)
			return nil, pb.ErrorCode_ERR_ITEM_NOT_ENOUGH
		}
		it.Count -= v.Count
		if it.Count == 0 {
			r.ItemRemove(v.Id)
		} else {
			r.trackItemMutation(v.Id, false, reason)
		}
		r.Debugf("reduce item {id: %v, cnt: %v, after: %v, reason:[%d|%d]}",
			v.Id, v.Count, it.Count, reason.Reason, reason.Scene)
	}
	return out, pb.ErrorCode_ERR_OK
}

// 当item数量为0时删除item
func (r *Role) ItemRemove(itemId int32) {
	if r.PbRole.InventoryInfo.ItemMap != nil {
		delete(r.PbRole.InventoryInfo.ItemMap, itemId)
	}
	r.MarkInventoryDirty(itemId, true)
}

// 添加单个道具
func (r *Role) ItemAdd(itemId int32, itemCount int64, reason *Reason) pb.ErrorCode {
	if itemCount == 0 {
		return pb.ErrorCode_ERR_OK
	}

	// 配置不存在的道具必须拒绝：ItemSee 对未知 ID 返回空列表，
	// 若不在此拦截会静默假成功（模拟测试 T14 发现）。
	if !r.Currency.IsCurrency(itemId) && itemconf.GetItemByItemId(itemId) == nil {
		return pb.ErrorCode_ERR_CONF
	}

	items := r.ItemSee(&pb.PbItem{Id: itemId, Count: itemCount})
	ret := pb.ErrorCode_ERR_OK
	for _, v := range *items {
		// itemDoAdd 的错误码必须透传（MaxOwnCount 超限、配置缺失等）：
		// 丢弃会让调用方拿到假成功（模拟测试 T13 发现）。
		if code := r.itemDoAdd(v.Id, v.Count, reason); code != 0 {
			ret = pb.ErrorCode(code)
		}
	}

	return ret
}

// 添加多个道具
func (r *Role) ItemsAdd(items *[]*pb.PbItem, reason *Reason) pb.ErrorCode {
	if items == nil || len(*items) == 0 {
		return 0
	}

	if ret := r.ItemsCheckAdd(items); 0 != ret {
		return ret
	}

	realItems := r.ItemsSee(items)
	ret := pb.ErrorCode_ERR_OK
	for _, v := range *realItems {
		if code := r.itemDoAdd(v.Id, v.Count, reason); code != 0 {
			ret = pb.ErrorCode(code)
		}
	}
	return ret
}

func (r *Role) itemDoAdd(itemId int32, itemCount int64, reason *Reason) int {
	if itemCount == 0 {
		return 0
	}

	// 货币（含经验/体力等资源）：统一入 currency_map，CurrencyComponent 内
	// 完成溢出保护、脏标记与回调。
	if r.Currency.IsCurrency(itemId) {
		return int(r.Currency.Add(itemId, itemCount, reason))
	}

	itemConf := itemconf.GetItemByItemId(itemId)
	if itemConf == nil {
		r.Errorf("conf not find, {id=%d}", itemId)
		return int(pb.ErrorCode_ERR_CONF)
	}

	if ret := r.ItemCheckAdd(itemId, itemCount); ret != 0 {
		return ret
	}

	// 获得即使用：道具规则 Getuse>0 时，AddItem 不入背包，直接转使用流程。
	// 典型场景：宝箱/礼包（拿到即开）、碎片（拿到即合成）。
	if rule := r.getItemRule(itemConf); rule != nil && rule.Getuse > 0 {
		useConf := itemconf.GetItemUseById(itemId)
		if useConf != nil && useConf.IsEnable != 0 {
			return int(r.useItemInner(useConf, int32(itemCount), reason))
		}
	}

	// 按MainType分层处理。
	// 图像道具（头像 701/70101、头像框 70102）的 EItemMainType_ICON 系枚举已随
	// 道具清理从 proto 移除，但存量角色数据仍可能携带 MainType=701，分支以
	// 历史字面量保留（IconAdd/FrameAdd 落 IconMap，不入背包 ItemMap）。
	switch itemConf.MainType {
	case 701:
		switch itemConf.SubType {
		case 70101:
			r.IconAdd(itemId, reason)
		case 70102:
			r.FrameAdd(itemId, reason)
		}

	default: // 通用背包物品处理
		it := r.PbRole.InventoryInfo.ItemMap[itemId]
		if it == nil {
			it = &pb.PbItem{Id: itemId}
			r.PbRole.InventoryInfo.ItemMap[itemId] = it
		}
		it.Count += itemCount
		r.trackItemMutation(itemId, false, reason)
	}

	r.Debugf("ITEM| add item {id: %v, count: %v, reason:[%v|%v]}", itemId, itemCount, reason.Reason, reason.Scene)
	return 0
}

func (r *Role) ItemExchange(consumeID int32, consumeCnt int64, productId int32, productCnt int64, reason *Reason) pb.ErrorCode {
	_, ret := r.ItemCheckReduce(consumeID, consumeCnt)
	if ret != pb.ErrorCode_ERR_OK {
		return ret
	}

	if _, ret = r.ItemReduce(consumeID, consumeCnt, reason); ret != pb.ErrorCode_ERR_OK {
		return ret
	}

	return r.ItemAdd(productId, productCnt, reason)
}

// TODO 道具使用后得到的新道具
func (r *Role) GetItemOutcomes(itemId int32) *[]*pb.PbItem {
	return nil
}

func (r *Role) DiamondCheckEnough(count int64) pb.ErrorCode {
	_, ret := r.ItemCheckReduce(int32(pb.EItemID_DIAMOND), count)
	return ret
}

func (r *Role) DiamondReduce(count int64, reason *Reason) pb.ErrorCode {
	_, ret := r.ItemReduce(int32(pb.EItemID_DIAMOND), count, reason)
	return ret
}

func (r *Role) DiamondAdd(count int64, reason *Reason) pb.ErrorCode {
	return r.ItemAdd(int32(pb.EItemID_DIAMOND), count, reason)
}

func (r *Role) GoldCheckEnough(count int64) pb.ErrorCode {
	_, ret := r.ItemCheckReduce(int32(pb.EItemID_GOLD), count)
	return ret
}

func (r *Role) GoldReduce(count int64, reason *Reason) pb.ErrorCode {
	_, ret := r.ItemReduce(int32(pb.EItemID_GOLD), count, reason)
	return ret
}

func (r *Role) GoldAdd(count int64, reason *Reason) pb.ErrorCode {
	return r.ItemAdd(int32(pb.EItemID_GOLD), count, reason)
}

// 将相同的id聚合在一起
func (r *Role) itemAggregate(items *[]*pb.PbItem) *[]*pb.PbItem {
	m := make(map[int32]*pb.PbItem)
	for _, v := range *items {
		if _, in := m[v.Id]; in {
			m[v.Id].Count += v.Count
		} else {
			m[v.Id] = v
		}
	}
	*items = (*items)[:0]
	for _, v := range m {
		*items = append(*items, v)
	}
	return items
}

// ============================================================
// 道具使用 / 出售 / 分解 / 背包查询
// ============================================================

// ItemUseType 道具使用类型（与 ItemUseConfig.UseType 对应）
const (
	itemUseTypeGainItem  int32 = 2 // 获得道具：UseId 作道具ID，UseValue 作数量
	itemUseTypeDropGroup int32 = 3 // 掉落组：UseId 作掉落组ID，UseValue 作次数
)

// ItemUse 主动使用道具：扣道具 → 按 UseType 发效果。
func (r *Role) ItemUse(itemId int32, count int32, reason *Reason) pb.ErrorCode {
	if count <= 0 {
		return pb.ErrorCode_ERR_ARGV
	}
	useConf := itemconf.GetItemUseById(itemId)
	if useConf == nil || useConf.IsEnable == 0 {
		return pb.ErrorCode_ERR_ITEM_CAN_NOT_USE
	}
	// 主动使用：先扣道具
	if _, ret := r.ItemReduce(itemId, int64(count), reason); ret != pb.ErrorCode_ERR_OK {
		return ret
	}
	return r.applyItemUse(useConf, count, reason)
}

// useItemInner 供 getuse 自动使用调用：道具未入背包，不扣，直接发效果。
func (r *Role) useItemInner(useConf *pb.ItemUseConfig, count int32, reason *Reason) pb.ErrorCode {
	return r.applyItemUse(useConf, count, reason)
}

// applyItemUse 按使用类型分发效果（只发不扣）。
func (r *Role) applyItemUse(useConf *pb.ItemUseConfig, count int32, reason *Reason) pb.ErrorCode {
	switch useConf.UseType {
	case itemUseTypeGainItem:
		// 获得道具：给 UseId × UseValue × count
		prodCnt := int64(useConf.UseValue) * int64(count)
		if prodCnt <= 0 {
			prodCnt = int64(count)
		}
		return r.ItemAdd(useConf.UseId, prodCnt, reason)
	case itemUseTypeDropGroup:
		// 掉落组：UseId 是掉落组ID，count 是次数。
		// DropExecute 内部完成"组→包"两层解析与 ItemAdd 落账；
		// UseId 不在组表时降级为按掉落包直接产出（drop_group.go executeDropOnce）。
		r.DropExecute(useConf.UseId, count, reason)
		return pb.ErrorCode_ERR_OK
	default:
		r.Errorf("ITEM|use unknown type {item:%d, type:%d}", useConf.Id, useConf.UseType)
		return pb.ErrorCode_ERR_ITEM_CAN_NOT_USE
	}
}

// ItemSell 出售道具：扣道具 + 加金币（金币 = ItemConfig.Sale × count）。
func (r *Role) ItemSell(itemId int32, count int32, reason *Reason) pb.ErrorCode {
	if count <= 0 {
		return pb.ErrorCode_ERR_ARGV
	}
	if r.Currency.IsCurrency(itemId) {
		return pb.ErrorCode_ERR_ITEM_CAN_NOT_SELL
	}
	itemConf := itemconf.GetItemByItemId(itemId)
	if itemConf == nil || itemConf.Sale <= 0 {
		return pb.ErrorCode_ERR_ITEM_CAN_NOT_SELL
	}
	if _, ret := r.ItemReduce(itemId, int64(count), reason); ret != pb.ErrorCode_ERR_OK {
		return ret
	}
	gold := int64(itemConf.Sale) * int64(count)
	return r.GoldAdd(gold, reason)
}

// ItemDecompose 分解道具：扣道具 → 按 DecomposeConfig 发放产出。
// 返回 (产出列表, 错误码)。
func (r *Role) ItemDecompose(itemId int32, count int32, reason *Reason) (*[]*pb.PbItem, pb.ErrorCode) {
	if count <= 0 {
		return nil, pb.ErrorCode_ERR_ARGV
	}
	if r.Currency.IsCurrency(itemId) {
		return nil, pb.ErrorCode_ERR_ITEM_CAN_NOT_DECOMPOSE
	}
	outputs := itemconf.GroupItemDecomposeByItemId(itemId)
	if len(outputs) == 0 {
		return nil, pb.ErrorCode_ERR_ITEM_CAN_NOT_DECOMPOSE
	}
	if _, ret := r.ItemReduce(itemId, int64(count), reason); ret != pb.ErrorCode_ERR_OK {
		return nil, ret
	}
	rewards := make([]*pb.PbItem, 0, len(outputs))
	for _, out := range outputs {
		if out.IsEnable == 0 {
			continue
		}
		prodCnt := int64(out.OutputCount) * int64(count)
		if prodCnt <= 0 {
			continue
		}
		if ret := r.ItemAdd(out.OutputItemId, prodCnt, reason); ret != pb.ErrorCode_ERR_OK {
			continue
		}
		rewards = append(rewards, &pb.PbItem{Id: out.OutputItemId, Count: prodCnt})
	}
	return &rewards, pb.ErrorCode_ERR_OK
}

// QueryBackpack 分页查询背包：按 bagType 过滤 + Quality/Id 排序 + 分页。
// bagType=0 表示全部分类；pageIdx 从 1 起；pageSize 0 用默认 30，上限 100。
func (r *Role) QueryBackpack(bagType int32, pageIdx int32, pageSize int32) (int32, []*pb.PbItem) {
	const defaultPageSize int32 = 30
	const maxPageSize int32 = 100
	if pageIdx <= 0 {
		pageIdx = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	views := r.buildBackpackViews(bagType)
	total := int32(len(views))
	start := (pageIdx - 1) * pageSize
	var items []*pb.PbItem
	if start < total {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = views[start:end]
	}
	return total, items
}

// buildBackpackViews 构建（过滤+排序后的）背包条目列表。
func (r *Role) buildBackpackViews(bagType int32) []*pb.PbItem {
	inv := r.PbRole.InventoryInfo
	if inv == nil || inv.ItemMap == nil {
		return nil
	}
	views := make([]*pb.PbItem, 0, len(inv.ItemMap))
	for id, it := range inv.ItemMap {
		if it == nil || it.Count <= 0 {
			continue
		}
		conf := itemconf.GetItemByItemId(id)
		if conf != nil && bagType != 0 && conf.BagType != bagType {
			continue
		}
		views = append(views, it)
	}
	// 排序：品质降序 → ItemId 升序（conf 缺失按品质 0）
	sortBackpackViews(views)
	return views
}

// sortBackpackViews 就地排序背包条目：品质降序优先，再按 ItemId 升序。
func sortBackpackViews(views []*pb.PbItem) {
	for i := 1; i < len(views); i++ {
		for j := i; j > 0; j-- {
			qj := itemQuality(views[j].Id)
			qj1 := itemQuality(views[j-1].Id)
			if qj > qj1 || (qj == qj1 && views[j].Id < views[j-1].Id) {
				views[j], views[j-1] = views[j-1], views[j]
			} else {
				break
			}
		}
	}
}

// itemQuality 取道具品质；配置缺失返回 0。
func itemQuality(itemId int32) int32 {
	if c := itemconf.GetItemByItemId(itemId); c != nil {
		return c.Quality
	}
	return 0
}

// ============================================================
// 背包段组件（增量档）
// ============================================================

type ItemComponent struct {
	sectionBase

	inventoryUpserts int32Set
	inventoryDeletes int32Set
}

func NewItemComponent() *ItemComponent { return &ItemComponent{} }

func (c *ItemComponent) Name() string { return "inventory" }

func (c *ItemComponent) Flag() pb.ERoleSectionFlag {
	return pb.ERoleSectionFlag_INVENTORY_INFO
}

func (c *ItemComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *ItemComponent) OnDestroy()           {}

func (c *ItemComponent) InitField(uid uint64) {
	if c.role.PbRole.InventoryInfo == nil {
		c.role.PbRole.InventoryInfo = &pb.RoleInventoryInfo{}
		c.role.PbRole.InventoryInfo.ItemMap = make(map[int32]*pb.PbItem)
	}
}

func (c *ItemComponent) Touch(reason string) {
	c.touch(pb.ERoleSectionFlag_INVENTORY_INFO, reason)
}

func (c *ItemComponent) MarkDirty(itemID int32, deleted bool) {
	c.role.markPatchSection(pb.ERoleSectionFlag_INVENTORY_INFO)
	if deleted {
		delInt32Value(&c.inventoryUpserts, itemID)
		setInt32Value(&c.inventoryDeletes, itemID)
	} else {
		delInt32Value(&c.inventoryDeletes, itemID)
		setInt32Value(&c.inventoryUpserts, itemID)
	}
	c.role.markPersistSectionDirty(pb.ERoleSectionFlag_INVENTORY_INFO, "inventory")
}

func (c *ItemComponent) BuildPatch(dst *pb.ScSyncUserDataV2) bool {
	patch := &pb.RoleInventoryPatch{}
	for _, itemID := range sortedInt32Values(c.inventoryUpserts) {
		if c.role.PbRole.InventoryInfo == nil || c.role.PbRole.InventoryInfo.ItemMap == nil {
			continue
		}
		if item := c.role.PbRole.InventoryInfo.ItemMap[itemID]; item != nil {
			patch.UpsertItems = append(patch.UpsertItems, item)
		}
	}
	patch.DeleteItemIds = sortedInt32Values(c.inventoryDeletes)
	if len(patch.UpsertItems) == 0 && len(patch.DeleteItemIds) == 0 {
		return false
	}
	dst.InventoryPatch = patch
	return true
}

func (c *ItemComponent) ClearDirty() {
	c.inventoryUpserts = nil
	c.inventoryDeletes = nil
}

func inventorySection() roleSection {
	return messageSection(pb.ERoleSectionFlag_INVENTORY_INFO, "inventory",
		func(i *pb.RoleInfo) *pb.RoleInventoryInfo { return i.InventoryInfo },
		func(i *pb.RoleInfo, m *pb.RoleInventoryInfo) { i.InventoryInfo = m },
		func() *pb.RoleInventoryInfo { return new(pb.RoleInventoryInfo) })
}
