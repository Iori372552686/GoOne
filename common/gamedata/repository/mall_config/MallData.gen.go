/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package mall_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type MallConfigData struct {
	_List []*protocol.MallConfig
	_Id   map[int32]*protocol.MallConfig
}

// 注册函数
func init() {
	gamedata.Register("MallConfig", parse)
}

func parse(buf string) error {
	data := &protocol.MallConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Id := make(map[int32]*protocol.MallConfig)
	for _, item := range data.Ary {
		_Id[item.Id] = item
	}

	obj.Store(&MallConfigData{
		_List: data.Ary,
		_Id:   _Id,
	})
	return nil
}

func GetHead() *protocol.MallConfig {
	obj, ok := obj.Load().(*MallConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.MallConfig) {
	obj, ok := obj.Load().(*MallConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.MallConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.MallConfig) bool) {
	obj, ok := obj.Load().(*MallConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetById(Id int32) *protocol.MallConfig {
	obj, ok := obj.Load().(*MallConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._Id[Id]; ok {
		return val
	}
	return nil
}
