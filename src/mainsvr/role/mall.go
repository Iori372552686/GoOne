package role

import g1_protocol "github.com/Iori372552686/GoOne/protobuf/protocol"

func (r *Role) MallGetItem(confId int32) *g1_protocol.PbMallItem {
	info := r.PbRole.MallInfo
	for _, v := range info.ItemList {
		if v.ConfId == confId {
			return v
		}
	}

	item := &g1_protocol.PbMallItem{ConfId: confId}
	info.ItemList = append(info.ItemList, item)
	return item
}

func (r *Role) MallDailyRefresh() {
	info := r.PbRole.MallInfo
	for _, v := range info.ItemList {
		v.DailyBuyCount = 0
	}
}

func (r *Role) MallAddBuyCount(confId int32) {
	item := r.MallGetItem(confId)
	item.DailyBuyCount++
	item.TotalBuyCount++
}

func (r *Role) MallCheckBuyCondition(confId int32) int {

	return 0
}
