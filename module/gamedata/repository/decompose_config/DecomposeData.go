package decompose_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type DecomposeConfigData struct {
	_List   []*protocol.DecomposeConfig
	_ItemId map[int32][]*protocol.DecomposeConfig
}

func init() {
	gamedata.Register("DecomposeConfig", parse)
}

func parse(buf string) error {
	data := &protocol.DecomposeConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_ItemId := make(map[int32][]*protocol.DecomposeConfig)
	for _, item := range data.Ary {
		_ItemId[item.ItemId] = append(_ItemId[item.ItemId], item)
	}

	obj.Store(&DecomposeConfigData{
		_List:   data.Ary,
		_ItemId: _ItemId,
	})
	return nil
}

func GetHead() *protocol.DecomposeConfig {
	d, ok := obj.Load().(*DecomposeConfigData)
	if !ok {
		return nil
	}
	if len(d._List) == 0 {
		return nil
	}
	return d._List[0]
}

func GetAll() []*protocol.DecomposeConfig {
	d, ok := obj.Load().(*DecomposeConfigData)
	if !ok {
		return nil
	}
	rets := make([]*protocol.DecomposeConfig, len(d._List))
	copy(rets, d._List)
	return rets
}

func Range(f func(*protocol.DecomposeConfig) bool) {
	d, ok := obj.Load().(*DecomposeConfigData)
	if !ok {
		return
	}
	for _, item := range d._List {
		if !f(item) {
			return
		}
	}
}

// GetByItemId 按"被分解道具Id"查询；一个道具可能有多条分解产出，返回全部启用项。
func GetByItemId(ItemId int32) []*protocol.DecomposeConfig {
	d, ok := obj.Load().(*DecomposeConfigData)
	if !ok {
		return nil
	}
	return d._ItemId[ItemId]
}
