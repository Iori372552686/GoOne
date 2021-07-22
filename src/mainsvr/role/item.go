/// 道具相关

package role

import (
	g1_protocol `GoOne/protobuf/protocol`
)

/// 玩家道具相关操作放在这里

// 获取道具数量
func (r *Role) GetItemCount(itemId int32) int32 {
	v := r.ItemGetCountRef(itemId)
	if v == nil {
		return 0
	}
	return *v
}

// 获得道具数量的引用
func (r *Role) ItemGetCountRef(itemId int32) *int32 {
	if itemId == int32(g1_protocol.EItemID_GOLD) {
		return &(r.PbRole.BasicInfo.Gold)
	} else if itemId == int32(g1_protocol.EItemID_DIAMOND) {
		return &(r.PbRole.BasicInfo.Diamond)
	} else if itemId == int32(g1_protocol.EItemID_SINEW) {
		return &(r.PbRole.BasicInfo.Sinew)
	} else if itemId == int32(g1_protocol.EItemID_LIVENESS) {
		return &(r.PbRole.BasicInfo.Liveness)
	} else if itemId == int32(g1_protocol.EItemID_GUILDGOLD) {
		return &(r.PbRole.BasicInfo.GuildCoin)
	}else {
		for i := range r.PbRole.InventoryInfo.ItemList {
			if r.PbRole.InventoryInfo.ItemList[i].Id == itemId {
				return &(r.PbRole.InventoryInfo.ItemList[i].Count)
			}
		}
	}
	return nil
}

func (r *Role) ItemCheckAdd(itemId, itemCount int32) int {
	return 0
}

func (r *Role) ItemsCheckAdd(items *[]*g1_protocol.PbItem) int {
	for _, v := range *items {
		if 0 != r.ItemCheckAdd(v.Id, v.Count) {
			return int(g1_protocol.ErrorCode_ERR_ITEM_ADD_ERROR)
		}
	}
	return 0
}

// 生成掉落（如果输入列表里面有drop类型的道具，则展开drop）
func (r *Role) ItemsSee(in *[]*g1_protocol.PbItem) *[]*g1_protocol.PbItem {
	out := make([]*g1_protocol.PbItem, 0)

	for _, v := range *in {
		itemOut := r.ItemSee(v)
		out = append(out, *itemOut...)
	}
	return &out
}

// 生成掉落（如果输入列表里面有drop类型的道具，则展开drop）
func (r *Role) ItemSee(item *g1_protocol.PbItem) *[]*g1_protocol.PbItem {
	return nil
}

// 返回道具消耗后获得的新道具，比如开宝箱消耗一个宝箱，获得宝箱内的道具
func (r *Role) ItemCheckReduce(itemId, itemCount int32) (*[]*g1_protocol.PbItem, int) {
	have := r.GetItemCount(itemId)
	outcomes := r.GetItemOutcomes(itemId)
	ret := 0
	if have < itemCount {
		ret = int(g1_protocol.ErrorCode_ERR_ITEM_NOT_ENOUGH)
		if itemId == int32(g1_protocol.EItemID_GOLD) {
			ret = int(g1_protocol.ErrorCode_ERR_GOLD_NOT_ENOUGH)
		} else if itemId == int32(g1_protocol.EItemID_DIAMOND) {
			ret = int(g1_protocol.ErrorCode_ERR_DIAMOND_NOT_ENOUGH)
		} else if itemId == int32(g1_protocol.EItemID_SINEW) {
			ret = int(g1_protocol.ErrorCode_ERR_SINEW_NOT_ENOUGH)
		}
	}
	return outcomes, ret
}

func (r *Role) ItemsCheckReduce(items *[]*g1_protocol.PbItem) (*[]*g1_protocol.PbItem, int) {
	items = r.itemAggregate(items)
	outcomes := make([]*g1_protocol.PbItem, 0)
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

	return &outcomes, 0
}

// 反回实际添加的东西
func (r *Role) ItemReduce(itemId, itemCount int32, reason *Reason) (*[]*g1_protocol.PbItem, int) {
	if itemCount == 0 {
		return nil, 0
	}

	out, ret := r.ItemCheckReduce(itemId, itemCount)
	if 0 != ret {
		return nil, ret
	}

	ref := r.ItemGetCountRef(itemId)
	*ref -= itemCount

	if *ref == 0 {
		r.ItemRemove(itemId)
	}

	r.Debugf("reduce item {id: %v, cnt: %v, after: %v, reason:[%d|%d]}",
		itemId, itemCount, *ref, reason.Reason, reason.SubReason)

	return out, 0
}

func (r *Role) ItemsReduce(items *[]*g1_protocol.PbItem, reason *Reason) (*[]*g1_protocol.PbItem, int) {
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
		ref := r.ItemGetCountRef(v.Id)
		if ref == nil {
			r.Errorf("get ref nul {id: %v}", v.Id)
			return nil, -1
		}
		*ref -= v.Count
		if *ref == 0 {
			r.ItemRemove(v.Id)
		}
		r.Debugf("reduce item {id: %v, cnt: %v, after: %v, reason:[%d|%d]}",
			v.Id, v.Count, *ref, reason.Reason, reason.SubReason)
	}
	return out, 0
}

// 当item数量为0时删除item
func (r *Role) ItemRemove(itemId int32) int {
	inventory := r.PbRole.InventoryInfo
	for i, v := range inventory.ItemList {
		if v.Id == itemId {
			last := len(inventory.ItemList) - 1
			if i != last {
				inventory.ItemList[i], inventory.ItemList[last] = inventory.ItemList[last], inventory.ItemList[i]
			}
			inventory.ItemList = inventory.ItemList[0:last]
			break
		}
	}
	return 0
}

func (r *Role) ItemAdd(itemId, itemCount int32, reason *Reason) int {
	return 0
}

func (r *Role) ItemsAdd(items *[]*g1_protocol.PbItem, reason *Reason) int {
	if items == nil || len(*items) == 0 {
		return 0
	}

	if ret := r.ItemsCheckAdd(items); 0 != ret {
		return ret
	}

	realItems := r.ItemsSee(items)
	for _, v := range *realItems {
		r.itemDoAdd(v.Id, v.Count, reason)
	}
	return 0
}

func (r *Role) itemDoAdd(itemId, itemCount int32, reason *Reason) int {
	return 0
}

func (r *Role) ItemExchange(consumeID, consumeCnt, productId, productCnt int32, reason *Reason) int {
	_, ret := r.ItemCheckReduce(consumeID, consumeCnt)
	if ret != 0 {
		return ret
	}
	r.ItemReduce(consumeID, consumeCnt, reason)
	return r.ItemAdd(productId, productCnt, reason)
}

// TODO 道具使用后得到的新道具
func (r *Role) GetItemOutcomes(itemId int32) *[]*g1_protocol.PbItem {
	return nil
}

func (r *Role) SinewCheckEnough(count int32) int {
	_, ret := r.ItemCheckReduce(int32(g1_protocol.EItemID_SINEW), count)
	return ret
}

func (r *Role) SinewReduce(count int32, reason *Reason) int {
	_, ret := r.ItemReduce(int32(g1_protocol.EItemID_SINEW), count, reason)
	return ret
}

func (r *Role) SinewAdd(count int32, reason *Reason) int {
	return r.ItemAdd(int32(g1_protocol.EItemID_SINEW), count, reason)
}

func (r *Role) DiamondCheckEnough(count int32) int {
	_, ret := r.ItemCheckReduce(int32(g1_protocol.EItemID_DIAMOND), count)
	return ret
}

func (r *Role) DiamondReduce(count int32, reason *Reason) int {
	_, ret := r.ItemReduce(int32(g1_protocol.EItemID_DIAMOND), count, reason)
	return ret
}

func (r *Role) DiamondAdd(count int32, reason *Reason) int {
	return r.ItemAdd(int32(g1_protocol.EItemID_DIAMOND), count, reason)
}

func (r *Role) GoldCheckEnough(count int32) int {
	_, ret := r.ItemCheckReduce(int32(g1_protocol.EItemID_GOLD), count)
	return ret
}

func (r *Role) GoldReduce(count int32, reason *Reason) int {
	_, ret := r.ItemReduce(int32(g1_protocol.EItemID_GOLD), count, reason)
	return ret
}

func (r *Role) GoldAdd(count int32, reason *Reason) int {
	return r.ItemAdd(int32(g1_protocol.EItemID_GOLD), count, reason)
}

// 将相同的id聚合在一起
func (r *Role) itemAggregate(items *[]*g1_protocol.PbItem) *[]*g1_protocol.PbItem {
	m := make(map[int32]*g1_protocol.PbItem)
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
