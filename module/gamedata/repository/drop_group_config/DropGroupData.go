package drop_group_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type DropGroupConfigData struct {
	_List     []*protocol.DropGroupConfig
	_Groupid  map[int32][]*protocol.DropGroupConfig
}

func init() {
	gamedata.Register("DropGroupConfig", parse)
}

func parse(buf string) error {
	data := &protocol.DropGroupConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Groupid := make(map[int32][]*protocol.DropGroupConfig)
	for _, item := range data.Ary {
		_Groupid[item.Groupid] = append(_Groupid[item.Groupid], item)
	}

	obj.Store(&DropGroupConfigData{
		_List:    data.Ary,
		_Groupid: _Groupid,
	})
	return nil
}

func GetHead() *protocol.DropGroupConfig {
	d, ok := obj.Load().(*DropGroupConfigData)
	if !ok {
		return nil
	}
	if len(d._List) == 0 {
		return nil
	}
	return d._List[0]
}

func GetAll() []*protocol.DropGroupConfig {
	d, ok := obj.Load().(*DropGroupConfigData)
	if !ok {
		return nil
	}
	rets := make([]*protocol.DropGroupConfig, len(d._List))
	copy(rets, d._List)
	return rets
}

func Range(f func(*protocol.DropGroupConfig) bool) {
	d, ok := obj.Load().(*DropGroupConfigData)
	if !ok {
		return
	}
	for _, item := range d._List {
		if !f(item) {
			return
		}
	}
}

// GetByGroupid 按掉落组Id查询；一个组可能有多行(Subid不同)，返回全部。
func GetByGroupid(Groupid int32) []*protocol.DropGroupConfig {
	d, ok := obj.Load().(*DropGroupConfigData)
	if !ok {
		return nil
	}
	return d._Groupid[Groupid]
}
