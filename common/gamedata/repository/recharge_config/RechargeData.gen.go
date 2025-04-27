/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package recharge_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type RechargeConfigData struct {
	_List []*protocol.RechargeConfig
	_Id   map[int32]*protocol.RechargeConfig
}

// 注册函数
func init() {
	gamedata.Register("RechargeConfig", parse)
}

func parse(buf string) error {
	data := &protocol.RechargeConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Id := make(map[int32]*protocol.RechargeConfig)
	for _, item := range data.Ary {
		_Id[item.Id] = item
	}

	obj.Store(&RechargeConfigData{
		_List: data.Ary,
		_Id:   _Id,
	})
	return nil
}

func GetHead() *protocol.RechargeConfig {
	obj, ok := obj.Load().(*RechargeConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.RechargeConfig) {
	obj, ok := obj.Load().(*RechargeConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.RechargeConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.RechargeConfig) bool) {
	obj, ok := obj.Load().(*RechargeConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetById(Id int32) *protocol.RechargeConfig {
	obj, ok := obj.Load().(*RechargeConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._Id[Id]; ok {
		return val
	}
	return nil
}
