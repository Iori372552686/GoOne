package item_use_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type ItemUseConfigData struct {
	_List []*protocol.ItemUseConfig
	_Id   map[int32]*protocol.ItemUseConfig
}

func init() {
	gamedata.Register("ItemUseConfig", parse)
}

func parse(buf string) error {
	data := &protocol.ItemUseConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Id := make(map[int32]*protocol.ItemUseConfig)
	for _, item := range data.Ary {
		_Id[item.Id] = item
	}

	obj.Store(&ItemUseConfigData{
		_List: data.Ary,
		_Id:   _Id,
	})
	return nil
}

func GetHead() *protocol.ItemUseConfig {
	d, ok := obj.Load().(*ItemUseConfigData)
	if !ok {
		return nil
	}
	if len(d._List) == 0 {
		return nil
	}
	return d._List[0]
}

func GetAll() []*protocol.ItemUseConfig {
	d, ok := obj.Load().(*ItemUseConfigData)
	if !ok {
		return nil
	}
	rets := make([]*protocol.ItemUseConfig, len(d._List))
	copy(rets, d._List)
	return rets
}

func Range(f func(*protocol.ItemUseConfig) bool) {
	d, ok := obj.Load().(*ItemUseConfigData)
	if !ok {
		return
	}
	for _, item := range d._List {
		if !f(item) {
			return
		}
	}
}

func GetById(Id int32) *protocol.ItemUseConfig {
	d, ok := obj.Load().(*ItemUseConfigData)
	if !ok {
		return nil
	}
	if val, ok := d._Id[Id]; ok {
		return val
	}
	return nil
}
