package role

import (
	"github.com/Iori372552686/GoOne/common/gamedata/repository/mall_config"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

func (r *Role) MallGetItem(confId int32) *g1_protocol.PbMallItem {
	info := r.PbRole.MallInfo

	if info.ItemMap == nil {
		info.ItemMap = make(map[int32]*g1_protocol.PbMallItem)
		if info.ItemMap[confId] == nil {
			info.ItemMap[confId] = &g1_protocol.PbMallItem{ConfId: confId}
		}
	}

	return info.ItemMap[confId]
}

func (r *Role) MallDailyRefresh() {
	info := r.PbRole.MallInfo
	for _, v := range info.ItemMap {
		v.DailyBuyCount = 0
	}
}

func (r *Role) MallAddBuyCount(confId int32) {
	item := r.MallGetItem(confId)
	item.DailyBuyCount++
	item.TotalBuyCount++
}

func (r *Role) MallCheckBuyCondition(confId int32) g1_protocol.ErrorCode {
	conf := mall_config.GetById(confId)
	if conf == nil {
		return g1_protocol.ErrorCode_ERR_CONF
	}

	/*	now := int64(r.Now())
		if (conf.BeginTime > 0 && now < conf.BeginTime) ||
			(conf.EndTime > 0 && now > conf.EndTime) {
			return g1_protocol.ErrorCode_ERR_MALL_OUT_OF_TIME
		}*/

	item := r.MallGetItem(confId)
	if item.DailyBuyCount >= conf.DailyBuyLimit && conf.DailyBuyLimit > 0 {
		return g1_protocol.ErrorCode_ERR_MALL_DAILY_LIMIT
	}

	if item.TotalBuyCount >= conf.BuyLimit && conf.BuyLimit > 0 {
		return g1_protocol.ErrorCode_ERR_MALL_BUY_LIMIT
	}

	return 0
}
