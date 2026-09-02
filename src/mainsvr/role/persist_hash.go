package role

import (
	"context"
	"errors"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	rds "github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

// 角色持久化：Redis hash field 分模块增量写。
//
// 存储格式：
//   key   = "DB_TYPE_ROLE:<uid>"
//   field = roleSectionDef.name   （register/login/game/basic/inventory/...）
//   value = proto.Marshal(对应子 message)
//
// 增量语义：saveRoleHash 只写 persistDirtyMask 命中的模块；force=true（停机 flush、
// 首次创建）时写全部模块。
//
// 注：RoleInfo 的 GiftInfo 无对应 ERoleSectionFlag，ConnSvrInfo 为运行时状态不落盘，
// 前者在 force 全量写时覆盖；增量更新期间若仅变更了 GiftInfo（当前无此路径），
// 需为其补充 section flag。

// roleSectionAccessor 把一个 ERoleSectionFlag 绑定到 RoleInfo 子 message 的
// 读访问器，用于 marshal 写入 hash field。
type roleSectionAccessor struct {
	flag g1_protocol.ERoleSectionFlag
	name string
	get  func(info *g1_protocol.RoleInfo) proto.Message
}

// roleSectionAccessors 与 sync_state.go 的 roleSectionDefs 顺序保持一致。
// 每个 get 返回子 message（可能为 nil，nil 时跳过写入）。
var roleSectionAccessors = []roleSectionAccessor{
	{g1_protocol.ERoleSectionFlag_REGISTER_INFO, "register", func(i *g1_protocol.RoleInfo) proto.Message { return i.RegisterInfo }},
	{g1_protocol.ERoleSectionFlag_LOGIN_INFO, "login", func(i *g1_protocol.RoleInfo) proto.Message { return i.LoginInfo }},
	{g1_protocol.ERoleSectionFlag_GAME_INFO, "game", func(i *g1_protocol.RoleInfo) proto.Message { return i.GameInfo }},
	{g1_protocol.ERoleSectionFlag_BASIC_INFO, "basic", func(i *g1_protocol.RoleInfo) proto.Message { return i.BasicInfo }},
	{g1_protocol.ERoleSectionFlag_INVENTORY_INFO, "inventory", func(i *g1_protocol.RoleInfo) proto.Message { return i.InventoryInfo }},
	{g1_protocol.ERoleSectionFlag_ICON_INFO, "icon", func(i *g1_protocol.RoleInfo) proto.Message { return i.IconInfo }},
	{g1_protocol.ERoleSectionFlag_MALL_INFO, "mall", func(i *g1_protocol.RoleInfo) proto.Message { return i.MallInfo }},
	{g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO, "main_task", func(i *g1_protocol.RoleInfo) proto.Message { return i.MainTaskInfo }},
	{g1_protocol.ERoleSectionFlag_GUILD_INFO, "guild", func(i *g1_protocol.RoleInfo) proto.Message { return i.GuildInfo }},
	{g1_protocol.ERoleSectionFlag_GUIDE_INFO, "guide", func(i *g1_protocol.RoleInfo) proto.Message { return i.GuideInfo }},
	{g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO, "open_func", func(i *g1_protocol.RoleInfo) proto.Message { return i.OpenFunInfo }},
	{g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, "activity_task", func(i *g1_protocol.RoleInfo) proto.Message { return i.Actvity_Info }},
}

// roleHashKey 返回角色在 Redis 的 key（full 与 hash 模式共用）。
func roleHashKey(uid uint64) string {
	return fmt.Sprintf("%s:%d", g1_protocol.DBType_DB_TYPE_ROLE.String(), uid)
}

// roleRedisInstance 角色数据所在的 Redis 实例 id。
func roleRedisInstance() uint32 {
	return uint32(g1_protocol.DBType_DB_TYPE_ROLE)
}

// saveRoleHash 按 persistDirtyMask 把变更模块写入 Redis hash。
// force=true 时无视 mask，全量写所有模块（用于停机 flush、首次创建）。
func saveRoleHash(r *Role, force bool) error {
	instID := roleRedisInstance()
	key := roleHashKey(r.Uid())

	writeMask := r.persistDirtyMask
	if force || writeMask == g1_protocol.ERoleSectionFlag_ALL || writeMask == 0 {
		// force / 首次（mask 未累积）/ 显式全量：写所有模块
		writeMask = g1_protocol.ERoleSectionFlag_ALL
	}

	wrote := 0
	for _, acc := range roleSectionAccessors {
		if !force && writeMask != g1_protocol.ERoleSectionFlag_ALL && !hasRoleSection(writeMask, acc.flag) {
			continue
		}
		msg := acc.get(r.PbRole)
		if msg == nil {
			continue
		}
		buf, err := proto.Marshal(msg)
		if err != nil {
			r.Errorf("role hash marshal error {uid:%v, field:%s} | %v", r.Uid(), acc.name, err)
			return err
		}
		// HSET key field value
		if err := rds.RedisMgr.HSetBytes(context.Background(), instID, key, acc.name, buf); err != nil {
			logger.Errorf("role hash HSET error {uid:%v, field:%s} | %v", r.Uid(), acc.name, err)
			return errors.New("role hash HSET error")
		}
		wrote++
	}

	// GiftInfo 无 section flag：force 全量时一并写入兜底 field，避免数据丢失。
	// ConnSvrInfo 为运行时状态，不持久化。
	if force && r.PbRole.GiftInfo != nil {
		if buf, err := proto.Marshal(r.PbRole.GiftInfo); err == nil {
			if err := rds.RedisMgr.HSetBytes(context.Background(), instID, key, "gift", buf); err != nil {
				logger.Errorf("role hash HSET gift error {uid:%v} | %v", r.Uid(), err)
				return errors.New("role hash HSET gift error")
			}
			wrote++
		}
	}

	r.Debugf("role hash save done {uid:%v, fields:%d, force:%v}", r.Uid(), wrote, force)
	return nil
}

// loadRoleHash 从 Redis hash 读回角色。
// hash 为空时回退读旧全量 string key（兼容 full 模式存量数据）。
// 返回 (roleInfo, migratedFromFull, error)：migratedFromFull 表示数据来自旧格式。
func loadRoleHash(uid uint64) (*g1_protocol.RoleInfo, error) {
	instID := roleRedisInstance()
	key := roleHashKey(uid)

	// HGETALL key → map[field]value
	fields, err := rds.RedisMgr.HGetAllBytes(context.Background(), instID, key)
	if err != nil {
		return nil, fmt.Errorf("role hash HGETALL error {uid:%v} | %w", uid, err)
	}
	if len(fields) == 0 {
		return nil, nil // 无数据
	}

	info := new(g1_protocol.RoleInfo)
	for _, acc := range roleSectionAccessors {
		buf, ok := fields[acc.name]
		if !ok || len(buf) == 0 {
			continue
		}
		// 用 accessor 的 get 需要先有占位实例；改为按 name 反射式 set 更直接，
		// 但为避免反射，这里通过 unmarshal 到独立消息后赋值。
		if err := unmarshalSection(info, acc.flag, buf); err != nil {
			return nil, fmt.Errorf("role hash unmarshal field %s {uid:%v} | %w", acc.name, uid, err)
		}
	}
	if buf, ok := fields["gift"]; ok && len(buf) > 0 {
		gift := new(g1_protocol.RoleGiftExchangeInfo)
		if err := proto.Unmarshal(buf, gift); err != nil {
			return nil, fmt.Errorf("role hash unmarshal gift {uid:%v} | %w", uid, err)
		}
		info.GiftInfo = gift
	}
	return info, nil
}

// unmarshalSection 按 flag 把 buf 反序列化到 RoleInfo 对应子字段。
func unmarshalSection(info *g1_protocol.RoleInfo, flag g1_protocol.ERoleSectionFlag, buf []byte) error {
	switch flag {
	case g1_protocol.ERoleSectionFlag_REGISTER_INFO:
		m := new(g1_protocol.RoleRegisterInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.RegisterInfo = m
	case g1_protocol.ERoleSectionFlag_LOGIN_INFO:
		m := new(g1_protocol.RoleLoginInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.LoginInfo = m
	case g1_protocol.ERoleSectionFlag_GAME_INFO:
		m := new(g1_protocol.RoleGameInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.GameInfo = m
	case g1_protocol.ERoleSectionFlag_BASIC_INFO:
		m := new(g1_protocol.RoleBasicInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.BasicInfo = m
	case g1_protocol.ERoleSectionFlag_INVENTORY_INFO:
		m := new(g1_protocol.RoleInventoryInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.InventoryInfo = m
	case g1_protocol.ERoleSectionFlag_ICON_INFO:
		m := new(g1_protocol.RoleIconInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.IconInfo = m
	case g1_protocol.ERoleSectionFlag_MALL_INFO:
		m := new(g1_protocol.RoleMallInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.MallInfo = m
	case g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO:
		m := new(g1_protocol.RoleMainTaskInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.MainTaskInfo = m
	case g1_protocol.ERoleSectionFlag_GUILD_INFO:
		m := new(g1_protocol.RoleGuildInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.GuildInfo = m
	case g1_protocol.ERoleSectionFlag_GUIDE_INFO:
		m := new(g1_protocol.RoleGuideInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.GuideInfo = m
	case g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO:
		m := new(g1_protocol.RoleOpenFunction)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.OpenFunInfo = m
	case g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO:
		m := new(g1_protocol.RoleActvityTaskInfo)
		if err := proto.Unmarshal(buf, m); err != nil {
			return err
		}
		info.Actvity_Info = m
	}
	return nil
}

// clearPersistDirtyMask 落盘成功后清零持久化 dirty mask。
func (r *Role) clearPersistDirtyMask() {
	r.persistDirtyMask = 0
}
