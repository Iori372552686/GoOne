package texas_room

import (
	"context"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	rds "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/rds"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/room_mgr/texas_room/texas"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

// roomRedisKey 拼装单个房间快照在 Redis 的 key。
// 用 index(zone) + stage 唯一定位，避免不同 zone 同 stage 冲突。
func roomRedisKey(index uint64, stage int32) string {
	return fmt.Sprintf("%s:%d:%d", g1_protocol.DBType_DB_TYPE_TEXAS_ROOM.String(), index, stage)
}

// SaveRoomDataToDB 把有变更的房间快照写入 Redis（周期持久化，10s 节拍驱动）。
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
		if code := saveStageSnapshot(instID, impl.Index, stage, roomInfo); code {
			saved++
		}
	}

	if saved > 0 {
		logger.Debugf("room snapshot saved {index:%d, count:%d}", impl.Index, saved)
	}
	return nil
}

// FlushAllRoomsToDB 强制全量写所有房间（停机 Drain 用）。无视 dirty 标志。
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
		if err := rds.RedisMgr.SetBytes(context.Background(), instID, key, buf, 0); err != nil {
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

// saveStageSnapshot 序列化并写入单个 stage 的快照，成功返回 true 并清 dirty。
// 调用方必须已持有读锁。
func saveStageSnapshot(instID uint32, index uint64, stage int32, roomInfo *texas.TexasRoom) bool {
	data := roomInfo.Get()
	if data == nil {
		return false
	}
	buf, err := proto.Marshal(data)
	if err != nil {
		logger.Errorf("marshal room snapshot error {index:%d, stage:%d} | %v", index, stage, err)
		return false
	}
	key := roomRedisKey(index, stage)
	if err := rds.RedisMgr.SetBytes(context.Background(), instID, key, buf, 0); err != nil {
		logger.Errorf("save room snapshot error {key:%s} | %v", key, err)
		return false
	}
	roomInfo.MarkSaved()
	return true
}

// ----------------------------------------------public----------------------------------------------

// GetTexasObj 获取指定 stage 的房间表；不存在则懒创建，并在创建临界区内
// 同步尝试恢复 Redis 快照（修复历史缺陷：旧实现在启动时 LoadAllFromDB，
// 但启动时刻 TexasMap/TexasMap[stage] 均为空 —— zone/stage 只能由请求懒创建 ——
// 导致恢复遍历的永远是空集合，Redis 里精心保存的快照重启后永远读不回）。
//
// 懒恢复的设计取舍：
//   - 精确性：首次触达某个 zone/stage 的请求才恢复，天然只恢复本实例负责的
//     分片，不会把其它实例的 zone 加载成"幽灵 zone"；
//   - 原子性：恢复在创建临界区（写锁）内完成，保证"快照恢复"与"首个业务写"
//     严格串行，不会被 Set 整表替换掉并发插入的房间；
//   - 代价：首次触达多一次 Redis 读（毫秒级），且持锁做 IO 仅发生一次；
//   - 降级：Redis 不可用/无快照时保持空表，由 gamesvr 后续上报重建，不阻塞启动。
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
			// 创建临界区内同步恢复快照（见函数注释"原子性"一节）。
			impl.restoreStageLocked(stage, data)
		}

		impl.Unlock()
	} else {
		impl.RUnlock()
	}

	return data
}

// restoreStageLocked 从 Redis 恢复指定 stage 的房间快照。
// 调用方必须已持有写锁。恢复成功后立即清 dirty（数据本身已落盘，无需回写）。
func (impl *TexasRoomCenterMgr) restoreStageLocked(stage int32, room *texas.TexasRoom) {
	if rds.RedisMgr.InstanceCount() == 0 {
		return
	}

	instID := uint32(g1_protocol.DBType_DB_TYPE_TEXAS_ROOM)
	key := roomRedisKey(impl.Index, stage)
	buf, err := rds.RedisMgr.GetBytes(context.Background(), instID, key)
	if err != nil {
		// Redis 异常不阻塞创建：保持空表，由 gamesvr 上报重建。
		logger.Errorf("restore room snapshot redis error {key:%s} | %v", key, err)
		return
	}
	if buf == nil {
		return // 无历史快照
	}

	data := new(g1_protocol.DBTexasRoomCenterInfo)
	if err := proto.Unmarshal(buf, data); err != nil {
		logger.Errorf("unmarshal room snapshot error {key:%s} | %v", key, err)
		return
	}
	if err := room.Set(data); err == nil {
		room.MarkSaved()
		logger.Infof("room snapshot restored {index:%d, stage:%d, rooms:%d}", impl.Index, stage, len(data.RoomsMap))
	}
}
