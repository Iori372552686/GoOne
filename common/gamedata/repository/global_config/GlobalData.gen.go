/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package global_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type GlobalConfigData struct {
	_List []*protocol.GlobalConfig
	_Name map[string]*protocol.GlobalConfig
}

// 注册函数
func init() {
	gamedata.Register("GlobalConfig", parse)
}

func parse(buf string) error {
	data := &protocol.GlobalConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Name := make(map[string]*protocol.GlobalConfig)
	for _, item := range data.Ary {
		_Name[item.Name] = item
	}

	obj.Store(&GlobalConfigData{
		_List: data.Ary,
		_Name: _Name,
	})
	return nil
}

func GetHead() *protocol.GlobalConfig {
	obj, ok := obj.Load().(*GlobalConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.GlobalConfig) {
	obj, ok := obj.Load().(*GlobalConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.GlobalConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.GlobalConfig) bool) {
	obj, ok := obj.Load().(*GlobalConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByName(Name string) *protocol.GlobalConfig {
	obj, ok := obj.Load().(*GlobalConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._Name[Name]; ok {
		return val
	}
	return nil
}
