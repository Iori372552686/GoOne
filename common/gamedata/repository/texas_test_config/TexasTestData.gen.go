/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package texas_test_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type TexasTestConfigData struct {
	_List  []*protocol.TexasTestConfig
	_Round map[uint32]*protocol.TexasTestConfig
}

// 注册函数
func init() {
	gamedata.Register("TexasTestConfig", parse)
}

func parse(buf string) error {
	data := &protocol.TexasTestConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Round := make(map[uint32]*protocol.TexasTestConfig)
	for _, item := range data.Ary {
		_Round[item.Round] = item
	}

	obj.Store(&TexasTestConfigData{
		_List:  data.Ary,
		_Round: _Round,
	})
	return nil
}

func GetHead() *protocol.TexasTestConfig {
	obj, ok := obj.Load().(*TexasTestConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.TexasTestConfig) {
	obj, ok := obj.Load().(*TexasTestConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.TexasTestConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.TexasTestConfig) bool) {
	obj, ok := obj.Load().(*TexasTestConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByRound(Round uint32) *protocol.TexasTestConfig {
	obj, ok := obj.Load().(*TexasTestConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._Round[Round]; ok {
		return val
	}
	return nil
}
