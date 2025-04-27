/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package texas_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type TexasConfigData struct {
	_List              []*protocol.TexasConfig
	_RoomStageCoinType map[protocol.Index2[int32, int32]]*protocol.TexasConfig
}

// 注册函数
func init() {
	gamedata.Register("TexasConfig", parse)
}

func parse(buf string) error {
	data := &protocol.TexasConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_RoomStageCoinType := make(map[protocol.Index2[int32, int32]]*protocol.TexasConfig)
	for _, item := range data.Ary {
		_RoomStageCoinType[protocol.Index2[int32, int32]{item.RoomStage, item.CoinType}] = item
	}

	obj.Store(&TexasConfigData{
		_List:              data.Ary,
		_RoomStageCoinType: _RoomStageCoinType,
	})
	return nil
}

func GetHead() *protocol.TexasConfig {
	obj, ok := obj.Load().(*TexasConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.TexasConfig) {
	obj, ok := obj.Load().(*TexasConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.TexasConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.TexasConfig) bool) {
	obj, ok := obj.Load().(*TexasConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByRoomStageCoinType(RoomStage int32, CoinType int32) *protocol.TexasConfig {
	obj, ok := obj.Load().(*TexasConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._RoomStageCoinType[protocol.Index2[int32, int32]{RoomStage, CoinType}]; ok {
		return val
	}
	return nil
}
