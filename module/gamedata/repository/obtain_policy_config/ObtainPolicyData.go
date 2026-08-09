package obtain_policy_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/module/gamedata"
	protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type ObtainPolicyConfigData struct {
	_List   []*protocol.ObtainPolicyConfig
	_Source map[string]*protocol.ObtainPolicyConfig
}

func init() {
	gamedata.Register("ObtainPolicyConfig", parse)
}

func parse(buf string) error {
	data := &protocol.ObtainPolicyConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_Source := make(map[string]*protocol.ObtainPolicyConfig)
	for _, item := range data.Ary {
		_Source[item.Source] = item
	}

	obj.Store(&ObtainPolicyConfigData{
		_List:   data.Ary,
		_Source: _Source,
	})
	return nil
}

func GetHead() *protocol.ObtainPolicyConfig {
	d, ok := obj.Load().(*ObtainPolicyConfigData)
	if !ok {
		return nil
	}
	if len(d._List) == 0 {
		return nil
	}
	return d._List[0]
}

func GetAll() []*protocol.ObtainPolicyConfig {
	d, ok := obj.Load().(*ObtainPolicyConfigData)
	if !ok {
		return nil
	}
	rets := make([]*protocol.ObtainPolicyConfig, len(d._List))
	copy(rets, d._List)
	return rets
}

func Range(f func(*protocol.ObtainPolicyConfig) bool) {
	d, ok := obj.Load().(*ObtainPolicyConfigData)
	if !ok {
		return
	}
	for _, item := range d._List {
		if !f(item) {
			return
		}
	}
}

// GetBySource 按来源标识查询策略；未配置返回 nil。
func GetBySource(Source string) *protocol.ObtainPolicyConfig {
	d, ok := obj.Load().(*ObtainPolicyConfigData)
	if !ok {
		return nil
	}
	return d._Source[Source]
}
