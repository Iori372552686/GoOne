package role

import (
	"context"
	"fmt"
	"time"

	mysqlsvrv1 "github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/db/dal"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/conf"
	rds "github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

// 角色 DAL：lib/db/dal 泛型框架的角色适配层。
//
// 分层：
//   L1  RoleMgr.mapUidToRole（会话期工作集，本文件不涉及）
//   L2  Redis section-hash（persist_hash.go 的既有格式 + 可选 TTL）
//   L3  mysqlsvr role_data 整包快照（经 ssrpc）
//
// L3 读仅发生在登录回源（需要事务上下文走两路 RPC）；L3 写为 one-way 尽力
// 投递，由 Role 侧 needL3Flush 标记 + 防抖驱动（sync_state.go / role.go）。

// roleCodec 把 RoleInfo 在 L2 分段表示与 L3 整包表示之间转换。
type roleCodec struct{}

// MarshalL3 整包序列化。ConnSvrInfo 是运行时会话状态，落盘前必须清除
// （延续 saveRoleHash 的既有约定）；clone 隔离，不污染内存中的 Role。
func (roleCodec) MarshalL3(info *g1_protocol.RoleInfo) ([]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("marshal role l3: nil RoleInfo")
	}
	snapshot := proto.Clone(info).(*g1_protocol.RoleInfo)
	snapshot.ConnSvrInfo = nil
	return proto.Marshal(snapshot)
}

func (roleCodec) UnmarshalL3(data []byte) (*g1_protocol.RoleInfo, error) {
	info := new(g1_protocol.RoleInfo)
	if err := proto.Unmarshal(data, info); err != nil {
		return nil, err
	}
	return info, nil
}

// MarshalL2Fields sections 为 section 名列表；nil = 全量。
// 段集合来自 roleSectionRegistry（component.go，与 saveRoleHash/同步链路同源）；
// 未设置的段（真 nil）跳过，不写空 field。
func (roleCodec) MarshalL2Fields(info *g1_protocol.RoleInfo, sections []string) (map[string][]byte, error) {
	if info == nil {
		return nil, fmt.Errorf("marshal role l2: nil RoleInfo")
	}
	full := sections == nil
	wanted := make(map[string]bool, len(sections))
	for _, s := range sections {
		wanted[s] = true
	}

	fields := make(map[string][]byte, len(roleSectionRegistry))
	for i := range roleSectionRegistry {
		sec := &roleSectionRegistry[i]
		if !full && !wanted[sec.name] {
			continue
		}
		msg := sec.getMsg(info)
		if msg == nil {
			continue
		}
		buf, err := proto.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("marshal role l2 field %s: %w", sec.name, err)
		}
		fields[sec.name] = buf
	}
	return fields, nil
}

// UnmarshalL2Fields 按注册表把 hash field 集合反序列化（缺段零值补）。
func (roleCodec) UnmarshalL2Fields(fields map[string][]byte) (*g1_protocol.RoleInfo, error) {
	info := new(g1_protocol.RoleInfo)
	for i := range roleSectionRegistry {
		sec := &roleSectionRegistry[i]
		buf, ok := fields[sec.name]
		if !ok || len(buf) == 0 {
			continue
		}
		if err := sec.unmarshal(info, buf); err != nil {
			return nil, fmt.Errorf("unmarshal role l2 field %s: %w", sec.name, err)
		}
	}
	return info, nil
}

// redisRoleL2 L2 缓存后端：角色 section-hash。TTL 未配置（0）时保持现状
// 永不过期；配置后经 TxPipeline 一并提交 HSET+EXPIRE。
type redisRoleL2 struct{}

func (redisRoleL2) Load(ctx context.Context, uid uint64) (map[string][]byte, bool, error) {
	fields, err := rds.RedisMgr.HGetAllBytes(ctx, roleRedisInstance(), roleHashKey(uid))
	if err != nil {
		return nil, false, err
	}
	return fields, len(fields) > 0, nil
}

func (redisRoleL2) Save(ctx context.Context, uid uint64, fields map[string][]byte) error {
	return rds.RedisMgr.HSetFieldsExpire(ctx, roleRedisInstance(), roleHashKey(uid), fields, roleCacheTTL())
}

func (redisRoleL2) Delete(ctx context.Context, uid uint64) error {
	return rds.RedisMgr.Delete(ctx, roleRedisInstance(), roleHashKey(uid))
}

// mysqlRoleL3 L3 持久层后端：经 ssrpc 到 mysqlsvr。
// trans 仅两路 Load 需要（回源读）；Save 走 one-way，不需要事务上下文。
type mysqlRoleL3 struct {
	trans cmd_handler.IContext
}

func (l *mysqlRoleL3) Load(_ context.Context, uid uint64) ([]byte, int64, error) {
	if l.trans == nil {
		return nil, 0, fmt.Errorf("role l3 load: nil transaction context")
	}
	rsp, err := mysqlsvrClient().LoadRoleData(l.trans, &g1_protocol.MysqlInnerLoadRoleDataReq{Uid: uid})
	if err != nil {
		return nil, 0, err
	}
	if rsp == nil || rsp.Ret == nil || rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		code := int32(0)
		if rsp != nil && rsp.Ret != nil {
			code = int32(rsp.Ret.Code)
		}
		return nil, 0, fmt.Errorf("role l3 load uid=%d: ret code %d", uid, code)
	}
	return rsp.Data, rsp.UpdateTime, nil
}

// Save one-way 尽力投递：返回 error 仅代表投递失败（调用方保留 needL3Flush
// 重试）；落库乱序由 mysqlsvr 的 update_time 守卫兜底。
func (l *mysqlRoleL3) Save(_ context.Context, uid uint64, data []byte, updateTime int64) error {
	req := &g1_protocol.MysqlInnerSaveRoleDataReq{Uid: uid, Data: data, UpdateTime: updateTime, IgnoreRsp: true}
	return ssrpc.SendByCmdSimple(uid, 0, g1_protocol.CMD_MYSQL_INNER_SAVE_ROLE_DATA_REQ, req)
}

func (l *mysqlRoleL3) Delete(_ context.Context, uid uint64) error {
	// 角色删除仅清 L2；L3 快照保留为归档（与原方案 Delete 语义一致）。
	return nil
}

// RoleStore 角色 DAL 门面：组装 dal.TieredStore 并承载需要事务上下文的调用形态。
type RoleStore struct {
	codec roleCodec
	l2    redisRoleL2
	l3    dal.L3Store // 默认 &mysqlRoleL3{}；测试可注入桩
	clock func() int64
}

// NewRoleStore clock 为 nil 时用服务器墙钟 ms（禁止玩家时偏移时间，见 update_time 守卫）。
func NewRoleStore(clock func() int64) *RoleStore {
	return &RoleStore{l3: &mysqlRoleL3{}, clock: clock}
}

// setL3 供测试注入 L3 桩（生产保持 mysqlRoleL3）。
func (s *RoleStore) setL3(l3 dal.L3Store) { s.l3 = l3 }

func (s *RoleStore) tiered(trans cmd_handler.IContext) *dal.TieredStore[*g1_protocol.RoleInfo] {
	l3 := s.l3
	if m, ok := l3.(*mysqlRoleL3); ok {
		c := *m // 拷贝注入本次调用的事务上下文，不共享可变状态
		c.trans = trans
		l3 = &c
	}
	return dal.NewTieredStore[*g1_protocol.RoleInfo](s.codec, s.l2, l3, s.clock)
}

// Load 读穿：L2 → L3（需 trans）→ 回填 L2。found=false 表示全新角色。
func (s *RoleStore) Load(ctx context.Context, trans cmd_handler.IContext, uid uint64) (*g1_protocol.RoleInfo, bool, error) {
	return s.tiered(trans).Load(ctx, uid)
}

// SaveL2 写穿 L2：sections=nil 全量段，否则仅写指定段（saveRoleHash 的
// 唯一 L2 写入口，DAL 与运行期共用一条写路径）。
func (s *RoleStore) SaveL2(ctx context.Context, uid uint64, info *g1_protocol.RoleInfo, sections []string) error {
	return s.tiered(nil).SaveL2(ctx, uid, info, sections)
}

// SaveL3 整包快照 one-way 写回（尽力投递）。
func (s *RoleStore) SaveL3(ctx context.Context, uid uint64, info *g1_protocol.RoleInfo) error {
	return s.tiered(nil).SaveL3(ctx, uid, info)
}

// Delete 清 L2（L3 快照保留归档）。
func (s *RoleStore) Delete(ctx context.Context, uid uint64) error {
	return s.tiered(nil).Delete(ctx, uid)
}

// roleStore 包级实例：与 rds.RedisMgr 同风格的进程内单例，装配在包初始化
// （无配置依赖，clock 固定服务器墙钟）。
var roleStore = NewRoleStore(datetime.NowMs)

// saveRoleDataL3 角色 L3 快照写回（Role 方法内部使用）。
func saveRoleDataL3(uid uint64, info *g1_protocol.RoleInfo) error {
	if err := roleStore.SaveL3(context.Background(), uid, info); err != nil {
		logger.Errorf("role l3 flush send failed {uid:%d} | %v", uid, err)
		return err
	}
	return nil
}

// roleCacheTTL L2 缓存 TTL。未配置（0）= 永不过期（现状兼容，含全部单测）；
// 生产建议配置 30 天——前提是 L3 全量快照已启用。
func roleCacheTTL() time.Duration {
	days := conf.Get("mainsvr.capacity.role_cache_ttl_days").Int()
	if days <= 0 {
		return 0
	}
	return time.Duration(days) * 24 * time.Hour
}

// mysqlsvrClient 两路回源读的 ssrpc 客户端。
func mysqlsvrClient() *mysqlsvrv1.MysqlServiceClient {
	return mysqlsvrv1.NewMysqlServiceClient()
}
