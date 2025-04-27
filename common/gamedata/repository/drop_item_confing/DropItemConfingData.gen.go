/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package drop_item_confing

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type DropItemConfingData struct {
	_List       []*protocol.DropItemConfing
	_DropItemId map[int32]*protocol.DropItemConfing
}

// 注册函数
func init() {
	gamedata.Register("DropItemConfing", parse)
}

func parse(buf string) error {
	data := &protocol.DropItemConfingAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_DropItemId := make(map[int32]*protocol.DropItemConfing)
	for _, item := range data.Ary {
		_DropItemId[item.DropItemId] = item
	}

	obj.Store(&DropItemConfingData{
		_List:       data.Ary,
		_DropItemId: _DropItemId,
	})
	return nil
}

func GetHead() *protocol.DropItemConfing {
	obj, ok := obj.Load().(*DropItemConfingData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.DropItemConfing) {
	obj, ok := obj.Load().(*DropItemConfingData)
	if !ok {
		return
	}
	rets = make([]*protocol.DropItemConfing, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.DropItemConfing) bool) {
	obj, ok := obj.Load().(*DropItemConfingData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByDropItemId(DropItemId int32) *protocol.DropItemConfing {
	obj, ok := obj.Load().(*DropItemConfingData)
	if !ok {
		return nil
	}

	if val, ok := obj._DropItemId[DropItemId]; ok {
		return val
	}
	return nil
}
