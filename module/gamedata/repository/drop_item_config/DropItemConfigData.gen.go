/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package drop_item_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type DropItemConfigData struct {
	_List       []*protocol.DropItemConfig
	_DropItemId map[int32]*protocol.DropItemConfig
}

// 注册函数
func init() {
	gamedata.Register("DropItemConfig", parse)
}

func parse(buf string) error {
	data := &protocol.DropItemConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_DropItemId := make(map[int32]*protocol.DropItemConfig)
	for _, item := range data.Ary {
		_DropItemId[item.DropItemId] = item
	}

	obj.Store(&DropItemConfigData{
		_List:       data.Ary,
		_DropItemId: _DropItemId,
	})
	return nil
}

func GetHead() *protocol.DropItemConfig {
	obj, ok := obj.Load().(*DropItemConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.DropItemConfig) {
	obj, ok := obj.Load().(*DropItemConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.DropItemConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.DropItemConfig) bool) {
	obj, ok := obj.Load().(*DropItemConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByDropItemId(DropItemId int32) *protocol.DropItemConfig {
	obj, ok := obj.Load().(*DropItemConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._DropItemId[DropItemId]; ok {
		return val
	}
	return nil
}
