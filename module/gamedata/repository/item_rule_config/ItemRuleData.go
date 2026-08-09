package item_rule_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type ItemRuleConfigData struct {
	_List []*protocol.ItemRuleConfig
	_Id   map[int32]*protocol.ItemRuleConfig
}

func init() {
	gamedata.Register("ItemRuleConfig", parse)
}

func parse(buf string) error {
	data := &protocol.ItemRuleConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Id := make(map[int32]*protocol.ItemRuleConfig)
	for _, item := range data.Ary {
		_Id[item.Id] = item
	}

	obj.Store(&ItemRuleConfigData{
		_List: data.Ary,
		_Id:   _Id,
	})
	return nil
}

func GetHead() *protocol.ItemRuleConfig {
	d, ok := obj.Load().(*ItemRuleConfigData)
	if !ok {
		return nil
	}
	if len(d._List) == 0 {
		return nil
	}
	return d._List[0]
}

func GetAll() []*protocol.ItemRuleConfig {
	d, ok := obj.Load().(*ItemRuleConfigData)
	if !ok {
		return nil
	}
	rets := make([]*protocol.ItemRuleConfig, len(d._List))
	copy(rets, d._List)
	return rets
}

func Range(f func(*protocol.ItemRuleConfig) bool) {
	d, ok := obj.Load().(*ItemRuleConfigData)
	if !ok {
		return
	}
	for _, item := range d._List {
		if !f(item) {
			return
		}
	}
}

func GetById(Id int32) *protocol.ItemRuleConfig {
	d, ok := obj.Load().(*ItemRuleConfigData)
	if !ok {
		return nil
	}
	if val, ok := d._Id[Id]; ok {
		return val
	}
	return nil
}
