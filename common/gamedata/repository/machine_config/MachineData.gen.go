/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package machine_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type MachineConfigData struct {
	_List   []*protocol.MachineConfig
	_GameId map[int32]*protocol.MachineConfig
}

// 注册函数
func init() {
	gamedata.Register("MachineConfig", parse)
}

func parse(buf string) error {
	data := &protocol.MachineConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_GameId := make(map[int32]*protocol.MachineConfig)
	for _, item := range data.Ary {
		_GameId[item.GameId] = item
	}

	obj.Store(&MachineConfigData{
		_List:   data.Ary,
		_GameId: _GameId,
	})
	return nil
}

func GetHead() *protocol.MachineConfig {
	obj, ok := obj.Load().(*MachineConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.MachineConfig) {
	obj, ok := obj.Load().(*MachineConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.MachineConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.MachineConfig) bool) {
	obj, ok := obj.Load().(*MachineConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByGameId(GameId int32) *protocol.MachineConfig {
	obj, ok := obj.Load().(*MachineConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._GameId[GameId]; ok {
		return val
	}
	return nil
}
