package item_rule_ref_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type ItemRuleRefConfigData struct {
	_List   []*protocol.ItemRuleRefConfig
	_ItemId map[int32]*protocol.ItemRuleRefConfig
}

func init() {
	gamedata.Register("ItemRuleRefConfig", parse)
}

func parse(buf string) error {
	data := &protocol.ItemRuleRefConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_ItemId := make(map[int32]*protocol.ItemRuleRefConfig)
	for _, item := range data.Ary {
		_ItemId[item.ItemId] = item
	}

	obj.Store(&ItemRuleRefConfigData{
		_List:   data.Ary,
		_ItemId: _ItemId,
	})
	return nil
}

func GetHead() *protocol.ItemRuleRefConfig {
	d, ok := obj.Load().(*ItemRuleRefConfigData)
	if !ok {
		return nil
	}
	if len(d._List) == 0 {
		return nil
	}
	return d._List[0]
}

func GetAll() []*protocol.ItemRuleRefConfig {
	d, ok := obj.Load().(*ItemRuleRefConfigData)
	if !ok {
		return nil
	}
	rets := make([]*protocol.ItemRuleRefConfig, len(d._List))
	copy(rets, d._List)
	return rets
}

func Range(f func(*protocol.ItemRuleRefConfig) bool) {
	d, ok := obj.Load().(*ItemRuleRefConfigData)
	if !ok {
		return
	}
	for _, item := range d._List {
		if !f(item) {
			return
		}
	}
}

// GetByItemId 按道具Id查询其规则Id引用；未配置返回 nil。
func GetByItemId(ItemId int32) *protocol.ItemRuleRefConfig {
	d, ok := obj.Load().(*ItemRuleRefConfigData)
	if !ok {
		return nil
	}
	return d._ItemId[ItemId]
}
