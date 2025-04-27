/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package item_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type ItemConfigData struct {
	_List   []*protocol.ItemConfig
	_ItemId map[int32]*protocol.ItemConfig
}

// 注册函数
func init() {
	gamedata.Register("ItemConfig", parse)
}

func parse(buf string) error {
	data := &protocol.ItemConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_ItemId := make(map[int32]*protocol.ItemConfig)
	for _, item := range data.Ary {
		_ItemId[item.ItemId] = item
	}

	obj.Store(&ItemConfigData{
		_List:   data.Ary,
		_ItemId: _ItemId,
	})
	return nil
}

func GetHead() *protocol.ItemConfig {
	obj, ok := obj.Load().(*ItemConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.ItemConfig) {
	obj, ok := obj.Load().(*ItemConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.ItemConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.ItemConfig) bool) {
	obj, ok := obj.Load().(*ItemConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByItemId(ItemId int32) *protocol.ItemConfig {
	obj, ok := obj.Load().(*ItemConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._ItemId[ItemId]; ok {
		return val
	}
	return nil
}
