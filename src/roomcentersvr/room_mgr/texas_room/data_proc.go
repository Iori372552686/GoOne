package texas_room

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	rds "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room/texas"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"google.golang.org/protobuf/proto"
)

// roomRedisKey 拼装单个房间快照在 Redis 的 key。
// 用 index(zone) + stage 唯一定位，避免不同 zone 同 stage 冲突。
func roomRedisKey(index uint64, stage int32) string {
	return fmt.Sprintf("%s:%d:%d", g1_protocol.DBType_DB_TYPE_TEXAS_ROOM.String(), index, stage)
}

// SaveRoomDataToDB 把有变更的房间快照写入 Redis。
// 由 OnTick 周期触发（5s 节流在 RoomMgr.Tick 层）。
func (impl *TexasRoomCenterMgr) SaveRoomDataToDB() error {
	if !impl.checkOpen() {
		return nil
	}
	if rds.RedisMgr.InstanceCount() == 0 {
		return nil // 未配置 Redis，跳过持久化
	}

	instID := uint32(g1_protocol.DBType_DB_TYPE_TEXAS_ROOM)
	saved := 0

	impl.RLock()
	defer impl.RUnlock()

	for stage, roomInfo := range impl.TexasMap {
		if roomInfo == nil || !roomInfo.CheckChange() {
			continue
		}
		data := roomInfo.Get()
		if data == nil {
			continue
		}
		buf, err := proto.Marshal(data)
		if err != nil {
			logger.Errorf("marshal room snapshot error {index:%d, stage:%d} | %v", impl.Index, stage, err)
			continue
		}
		key := roomRedisKey(impl.Index, stage)
		if err := rds.RedisMgr.SetBytes(instID, key, buf); err != nil {
			logger.Errorf("save room snapshot error {key:%s} | %v", key, err)
			continue
		}
		// 写入成功后清 dirty（Update 内部会把 isChange 置 false）
		roomInfo.MarkSaved()
		saved++
	}

	if saved > 0 {
		logger.Debugf("room snapshot saved {index:%d, count:%d}", impl.Index, saved)
	}
	return nil
}

// FlushAllRoomsToDB 强制全量写所有房间（停机用）。无视 dirty 标志。
func (impl *TexasRoomCenterMgr) FlushAllRoomsToDB() (saved int, failed int) {
	if !impl.checkOpen() {
		return 0, 0
	}
	if rds.RedisMgr.InstanceCount() == 0 {
		return 0, 0
	}

	instID := uint32(g1_protocol.DBType_DB_TYPE_TEXAS_ROOM)

	impl.RLock()
	defer impl.RUnlock()

	for stage, roomInfo := range impl.TexasMap {
		if roomInfo == nil {
			continue
		}
		data := roomInfo.Get()
		if data == nil {
			continue
		}
		buf, err := proto.Marshal(data)
		if err != nil {
			logger.Errorf("flush room snapshot marshal error {index:%d, stage:%d} | %v", impl.Index, stage, err)
			failed++
			continue
		}
		key := roomRedisKey(impl.Index, stage)
		if err := rds.RedisMgr.SetBytes(instID, key, buf); err != nil {
			logger.Errorf("flush room snapshot error {key:%s} | %v", key, err)
			failed++
			continue
		}
		roomInfo.MarkSaved()
		saved++
	}

	logger.Infof("TexasRoomCenterMgr flush done {index:%d, saved:%d, failed:%d}", impl.Index, saved, failed)
	return saved, failed
}

// LoadRoomDataFromDB 从 Redis 恢复房间快照。启动时调用。
// 仅恢复当前 TexasMap 中已有 stage 的房间（stage 集合由配置/初始化决定）。
func (impl *TexasRoomCenterMgr) LoadRoomDataFromDB() error {
	if !impl.checkOpen() {
		return nil
	}
	if rds.RedisMgr.InstanceCount() == 0 {
		return nil
	}

	instID := uint32(g1_protocol.DBType_DB_TYPE_TEXAS_ROOM)
	restored := 0

	impl.Lock()
	defer impl.Unlock()

	for stage := range impl.TexasMap {
		key := roomRedisKey(impl.Index, stage)
		buf, err := rds.RedisMgr.GetBytes(instID, key)
		if err != nil {
			logger.Errorf("load room snapshot error {key:%s} | %v", key, err)
			continue
		}
		if buf == nil {
			continue // 无历史快照，跳过
		}
		data := new(g1_protocol.DBTexasRoomCenterInfo)
		if err := proto.Unmarshal(buf, data); err != nil {
			logger.Errorf("unmarshal room snapshot error {key:%s} | %v", key, err)
			continue
		}
		// 回填到现有 room 对象（Set 会更新 upTime 并置 dirty，恢复后立即清 dirty 避免误写）
		if room, ok := impl.TexasMap[stage]; ok && room != nil {
			if err := room.Set(data); err == nil {
				room.MarkSaved()
				restored++
			}
		}
	}

	if restored > 0 {
		logger.Infof("room snapshot restored {index:%d, count:%d}", impl.Index, restored)
	}
	return nil
}

// ----------------------------------------------public----------------------------------------------
func (impl *TexasRoomCenterMgr) GetTexasObj(stage int32) *texas.TexasRoom {
	var data *texas.TexasRoom
	var has bool

	impl.RLock()
	if data, has = impl.TexasMap[stage]; !has {
		impl.RUnlock()
		impl.Lock()

		//map  double-check
		if data, has = impl.TexasMap[stage]; !has {
			data = texas.NewTexasRoomObj(impl.Index, stage)
			impl.TexasMap[stage] = data
		}

		impl.Unlock()
	} else {
		impl.RUnlock()
	}

	return data
}

func (impl *TexasRoomCenterMgr) SetTexasRoom(data *g1_protocol.DBTexasRoomCenterInfo) error {
	if data == nil {
		return nil
	}
	return impl.GetTexasObj(int32(data.Stage)).Set(data)
}

func (impl *TexasRoomCenterMgr) OnCleanData(Stage int32) error {
	if !impl.checkOpen() || Stage == 0 {
		return nil
	}

	impl.Lock()
	delete(impl.TexasMap, Stage)
	impl.Unlock()

	// 清理 Redis 快照（如有）
	if rds.RedisMgr.InstanceCount() > 0 {
		_ = rds.RedisMgr.DelKey(uint32(g1_protocol.DBType_DB_TYPE_TEXAS_ROOM), roomRedisKey(impl.Index, Stage))
	}
	return nil
}
