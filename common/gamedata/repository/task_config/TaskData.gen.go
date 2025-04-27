/*
* 本代码由xlsx工具生成，请勿手动修改
 */

package task_config

import (
	"sync/atomic"

	"github.com/Iori372552686/GoOne/common/gamedata"
	protocol "github.com/gdsgog/poker_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

var obj = atomic.Value{}

type TaskConfigData struct {
	_List   []*protocol.TaskConfig
	_TaskID map[uint32]*protocol.TaskConfig
}

// 注册函数
func init() {
	gamedata.Register("TaskConfig", parse)
}

func parse(buf string) error {
	data := &protocol.TaskConfigAry{}
	if err := proto.UnmarshalText(buf, data); err != nil {
		return err
	}

	_TaskID := make(map[uint32]*protocol.TaskConfig)
	for _, item := range data.Ary {
		_TaskID[item.TaskID] = item
	}

	obj.Store(&TaskConfigData{
		_List:   data.Ary,
		_TaskID: _TaskID,
	})
	return nil
}

func GetHead() *protocol.TaskConfig {
	obj, ok := obj.Load().(*TaskConfigData)
	if !ok {
		return nil
	}
	return obj._List[0]
}

func GetAll() (rets []*protocol.TaskConfig) {
	obj, ok := obj.Load().(*TaskConfigData)
	if !ok {
		return
	}
	rets = make([]*protocol.TaskConfig, len(obj._List))
	copy(rets, obj._List)
	return
}

func Range(f func(*protocol.TaskConfig) bool) {
	obj, ok := obj.Load().(*TaskConfigData)
	if !ok {
		return
	}
	for _, item := range obj._List {
		if !f(item) {
			return
		}
	}
}

func GetByTaskID(TaskID uint32) *protocol.TaskConfig {
	obj, ok := obj.Load().(*TaskConfigData)
	if !ok {
		return nil
	}

	if val, ok := obj._TaskID[TaskID]; ok {
		return val
	}
	return nil
}
