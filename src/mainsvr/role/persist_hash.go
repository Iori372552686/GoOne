package role

import (
	"context"
	"fmt"

	rds "github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// 角色持久化：Redis hash field 分模块增量写。
//
// 存储格式：
//   key   = "DB_TYPE_ROLE:<uid>"
//   field = roleSection.name（component.go 注册表，单一来源）
//   value = proto.Marshal(对应子 message)
//
// 增量语义：saveRoleHash 只写 persistDirtyMask 命中的模块；force=true（停机 flush、
// 首次创建）时写全部模块。段 marshal/unmarshal 全部经 roleSectionRegistry，
// 与 DAL codec（store.go）同源。
//
// 注：GiftInfo 无对应 ERoleSectionFlag（注册表内 flag=0），仅在全量写时落盘；
// ConnSvrInfo 为运行时状态不落盘。

// roleHashKey 返回角色在 Redis 的 key（full 与 hash 模式共用）。
func roleHashKey(uid uint64) string {
	return fmt.Sprintf("%s:%d", g1_protocol.DBType_DB_TYPE_ROLE.String(), uid)
}

// roleRedisInstance 角色数据所在的 Redis 实例 id。
func roleRedisInstance() uint32 {
	return uint32(g1_protocol.DBType_DB_TYPE_ROLE)
}

// saveRoleHash 按 persistDirtyMask 把变更模块写入 Redis hash（F05：单命令原子提交）。
// force=true 时无视 mask，全量写所有模块（用于停机 flush、首次创建）。
// ctx 透传至底层 Redis 调用（F07）：调用方决定取消与预算语义。
//
// 提交方式：经 roleStore.SaveL2（DAL 写穿路径）一条多字段 HSET 提交——TTL 配置后
// HSET 与 EXPIRE 经 TxPipeline 一并生效，任一失败整批不提交（F05 语义的近似等价）；
// 失败保留 dirty mask 供重试。L2 成功即视为数据已过持久点，同时置 needL3Flush 供
// L3 快照防抖写回（见 store.go / sync_state.go）。
func saveRoleHash(ctx context.Context, r *Role, force bool) error {
	writeMask := r.persistDirtyMask
	if force || writeMask == g1_protocol.ERoleSectionFlag_ALL || writeMask == 0 {
		// force / 首次（mask 未累积）/ 显式全量：写所有模块（nil sections = 全量）
		writeMask = g1_protocol.ERoleSectionFlag_ALL
	}

	// sections=nil 走 codec 全量段；否则仅写 mask 命中的段名。
	var sections []string
	if writeMask != g1_protocol.ERoleSectionFlag_ALL {
		sections = marshalSectionsByMask(writeMask)
	}

	if err := roleStore.SaveL2(ctx, r.Uid(), r.PbRole, sections); err != nil {
		r.Errorf("role hash HSET fields error {uid:%v, sections:%d} | %v", r.Uid(), len(sections), err)
		return fmt.Errorf("role hash HSET fields error {uid:%v}: %w", r.Uid(), err)
	}

	// L2 已持久：标记 L3 待写回（MaybeFlushL3 按防抖消费此标记）。
	r.needL3Flush = true

	r.Debugf("role hash save done {uid:%v, full:%v}", r.Uid(), sections == nil)
	return nil
}

// loadRoleHash 从 Redis hash 读回角色（e2e 联调与排障入口；常规加载走
// roleStore.Load 的读穿路径，二者共用 roleSectionRegistry）。
// hash 为空（key 不存在或无字段）返回 (nil, nil) 表示无数据；
// 损坏字段（unmarshal 失败）作为整体失败返回错误，不以半新半旧状态发布。
func loadRoleHash(uid uint64) (*g1_protocol.RoleInfo, error) {
	instID := roleRedisInstance()
	key := roleHashKey(uid)

	fields, err := rds.RedisMgr.HGetAllBytes(context.Background(), instID, key)
	if err != nil {
		return nil, fmt.Errorf("role hash HGETALL error {uid:%v} | %w", uid, err)
	}
	if len(fields) == 0 {
		return nil, nil // 无数据
	}

	return decodeRoleHashFields(fields)
}

// decodeRoleHashFields 按 roleSectionRegistry 把 hash field 集合反序列化为 RoleInfo。
func decodeRoleHashFields(fields map[string][]byte) (*g1_protocol.RoleInfo, error) {
	info := new(g1_protocol.RoleInfo)
	for i := range roleSectionRegistry {
		sec := &roleSectionRegistry[i]
		buf, ok := fields[sec.name]
		if !ok || len(buf) == 0 {
			continue
		}
		if err := sec.unmarshal(info, buf); err != nil {
			return nil, fmt.Errorf("role hash unmarshal field %s | %w", sec.name, err)
		}
	}
	return info, nil
}

// clearPersistDirtyMask 落盘成功后清零持久化 dirty mask。
func (r *Role) clearPersistDirtyMask() {
	r.persistDirtyMask = 0
}
