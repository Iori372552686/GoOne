/// 掉落相关

package role

import (
	"math/rand"

	"github.com/Iori372552686/GoOne/common/gamedata/repository/drop_item_confing"
	"github.com/Iori372552686/GoOne/module/math"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

const (
	MAX_PROBABILITY = 10000
)

func (r *Role) DropGetItemByDropID(dropID int32) *[]*g1_protocol.PbItem {
	items := make([]*g1_protocol.PbItem, 0)

	r.Debugf("DROP|get drop: %d", dropID)
	dropByWeight := make([]*g1_protocol.DropItemConfing, 0)
	weightList := make([]int32, 0)
	drop_item_confing.Range(func(v *g1_protocol.DropItemConfing) bool {
		if v.DropId != dropID {
			return true
		}

		if v.DropWay == int32(g1_protocol.EItemDropWay_CERTAIN) { // 一定掉落
			item := g1_protocol.PbItem{Id: v.ItemId, Count: v.Count}
			items = append(items, &item)
			r.Debugf("DROP|add certain: %v", item)
		} else if v.DropWay == int32(g1_protocol.EItemDropWay_PROBABILITY) { // 概率掉落
			randV := int32(rand.Intn(MAX_PROBABILITY))
			if randV < v.Probability {
				item := g1_protocol.PbItem{Id: v.ItemId, Count: v.Count}
				items = append(items, &item)
				r.Debugf("DROP|add probability: %v", item)
			}
		} else if v.DropWay == int32(g1_protocol.EItemDropWay_WEIGHT) { // 分组权重掉落
			if v.GetProbability() == 0 {
				v.Probability = 1
			} //容错，没有得话默认1
			dropByWeight = append(dropByWeight, v)
			weightList = append(weightList, v.Probability)
			r.Debugf("DROP|add weight: %v", v)
		}

		return true
	})

	if len(dropByWeight) > 0 {
		idx := math.WeightedRandomSelect(weightList)
		item := g1_protocol.PbItem{Id: dropByWeight[idx].ItemId, Count: dropByWeight[idx].Count}
		items = append(items, &item)
	}

	r.Debugf("DROP| items: %v", items)
	return &items
}
