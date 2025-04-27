/// 道具相关

package role

import (
	"github.com/Iori372552686/GoOne/common/gamedata/repository/item_config"
	pb "github.com/Iori372552686/game_protocol/protocol"
)

/// 玩家道具相关操作放在这里

// 获取道具数量
func (r *Role) GetItemCount(itemId int32) int64 {
	v := r.ItemGetCountRef(itemId)
	if v == nil {
		return 0
	}
	return *v
}

// 获得道具数量的引用
func (r *Role) ItemGetCountRef(itemId int32) *int64 {
	switch itemId {
	case int32(pb.EItemID_GOLD):
		return &(r.PbRole.BasicInfo.Gold)
	case int32(pb.EItemID_DIAMOND):
		return &(r.PbRole.BasicInfo.Diamond)
	case int32(pb.EItemID_CREDIT):
		return &(r.PbRole.BasicInfo.Credit)
	case int32(pb.EItemID_LIVENESS):
		return &(r.PbRole.BasicInfo.Liveness)
	case int32(pb.EItemID_GUILDGOLD):
		return &(r.PbRole.BasicInfo.GuildCoin)
	case int32(pb.EItemID_ACECOIN):
		return &(r.PbRole.BasicInfo.AceCoin)
	case int32(pb.EItemID_WINACECOIN):
		return &(r.PbRole.BasicInfo.WinAceCoin)
	default:
		if r.PbRole.InventoryInfo.ItemMap[itemId] != nil {
			return &r.PbRole.InventoryInfo.ItemMap[itemId].Count
		}
	}

	return nil
}

func (r *Role) ItemCheckAdd(itemId int32, itemCount int64) int {
	return 0
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

// 生成掉落（如果输入列表里面有drop类型的道具，则展开drop）
func (r *Role) ItemSee(item *pb.PbItem) *[]*pb.PbItem {
	out := make([]*pb.PbItem, 0)

	conf := item_config.GetByItemId(item.Id)
	if conf == nil {
		return &out
	}
	if conf.Type == int32(pb.EItemType_DROP) {
		drop := r.DropGetItemByDropID(item.Id)
		out = append(out, *drop...)
	} else {
		out = append(out, item)
	}

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

	ref := r.ItemGetCountRef(itemId)
	*ref -= itemCount

	if *ref == 0 {
		r.ItemRemove(itemId)
	}

	r.Debugf("reduce item {id: %v, cnt: %v, after: %v, reason:[%d|%d]}",
		itemId, itemCount, *ref, reason.Reason, reason.Scene)

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
			v.Id, v.Count, *ref, reason.Reason, reason.Scene)
	}
	return out, pb.ErrorCode_ERR_OK
}

// 当item数量为0时删除item
func (r *Role) ItemRemove(itemId int32) {
	if r.PbRole.InventoryInfo.ItemMap != nil {
		delete(r.PbRole.InventoryInfo.ItemMap, itemId)
	}
}

// 添加单个道具
func (r *Role) ItemAdd(itemId int32, itemCount int64, reason *Reason) pb.ErrorCode {
	if itemCount == 0 {
		return pb.ErrorCode_ERR_OK
	}

	items := r.ItemSee(&pb.PbItem{Id: itemId, Count: itemCount})
	for _, v := range *items {
		r.itemDoAdd(v.Id, v.Count, reason)
	}

	return pb.ErrorCode_ERR_OK
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
	for _, v := range *realItems {
		r.itemDoAdd(v.Id, v.Count, reason)
	}
	return pb.ErrorCode_ERR_OK
}

func (r *Role) itemDoAdd(itemId int32, itemCount int64, reason *Reason) int {
	if itemCount == 0 {
		return 0
	}

	itemConf := item_config.GetByItemId(itemId)
	if itemConf == nil {
		r.Errorf("conf not find, {id=%d}", itemId)
		return int(pb.ErrorCode_ERR_CONF)
	}

	if ret := r.ItemCheckAdd(itemId, itemCount); ret != 0 {
		return ret
	}

	ref := r.ItemGetCountRef(itemId)
	switch pb.EItemID(itemId) {
	case pb.EItemID_GOLD,
		pb.EItemID_DIAMOND,
		pb.EItemID_LIVENESS,
		pb.EItemID_GUILDGOLD:
		*ref += itemCount

	case pb.EItemID_EXP: // 经验单独处理
		r.ExpAdd(itemCount)

	default:
		// 按MainType分层处理
		switch itemConf.MainType {
		case int32(pb.EItemMainType_ICON):
			switch itemConf.SubType {
			case int32(pb.EItemSubType_ICON_ICON):
				r.IconAdd(itemId, reason)
			case int32(pb.EItemSubType_ICON_FRAME):
				r.FrameAdd(itemId, reason)
			}

		default: // 通用背包物品处理
			if ref == nil {
				r.PbRole.InventoryInfo.ItemMap[itemId] = &pb.PbItem{Id: itemId, Count: 0}
				ref = &r.PbRole.InventoryInfo.ItemMap[itemId].Count
			}
			*ref += itemCount
		}
	}

	if ref != nil {
		//r.ActTaskReport(int32(pb.TaskName_TASK_GET_FIXED_ITEM), itemId, 0, 0, itemCount)
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

// ace coin
func (r *Role) AceCoinAdd(count int64, reason *Reason) pb.ErrorCode {
	return r.ItemAdd(int32(pb.EItemID_ACECOIN), count, reason)
}

func (r *Role) WinAceCoinAdd(count int64, reason *Reason) pb.ErrorCode {
	return r.ItemAdd(int32(pb.EItemID_WINACECOIN), count, reason)
}

func (r *Role) AceCoinCheckEnough(count int64) pb.ErrorCode {
	aceCnt := r.GetItemCount(int32(pb.EItemID_ACECOIN))
	winAceCnt := r.GetItemCount(int32(pb.EItemID_WINACECOIN))

	if aceCnt+winAceCnt < count {
		return pb.ErrorCode_ERR_ACE_COIN_NOT_ENOUGH
	}

	return pb.ErrorCode_ERR_OK
}

func (r *Role) AceCoinReduce(count int64, reason *Reason) pb.ErrorCode {
	aceCnt := r.GetItemCount(int32(pb.EItemID_ACECOIN))

	ret := pb.ErrorCode_ERR_ACE_COIN_NOT_ENOUGH
	if aceCnt >= count {
		_, ret = r.ItemReduce(int32(pb.EItemID_ACECOIN), count, reason)
	} else {
		_, ret = r.ItemReduce(int32(pb.EItemID_ACECOIN), aceCnt, reason)
		_, ret = r.ItemReduce(int32(pb.EItemID_WINACECOIN), count-aceCnt, reason)
	}

	return ret
}

func (r *Role) WinAceCoinReduce(count int64, reason *Reason) pb.ErrorCode {
	winAceCnt := r.GetItemCount(int32(pb.EItemID_WINACECOIN))

	ret := pb.ErrorCode_ERR_ACE_COIN_NOT_ENOUGH
	if winAceCnt >= count {
		_, ret = r.ItemReduce(int32(pb.EItemID_WINACECOIN), count, reason)
	} else {
		_, ret = r.ItemReduce(int32(pb.EItemID_WINACECOIN), winAceCnt, reason)
		_, ret = r.ItemReduce(int32(pb.EItemID_ACECOIN), count-winAceCnt, reason)
	}

	return ret
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
