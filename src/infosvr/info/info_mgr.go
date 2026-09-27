package info

import (
	"context"
	"fmt"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/lib/util/lru"
	"github.com/Iori372552686/GoOne/module/conf"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"

	"github.com/golang/protobuf/proto"
)

const (
	CACHE_SIZE = 10000
)

// InfoMgr 角色简要信息的 LRU 缓存 + Redis 持久化。
//
// 错误语义（F09）：
//   - 存储不可用：GetInfo/SetInfo 返回 ERR_DB——"存储故障"不再伪装成"玩家不存在"。
//   - 损坏数据（unmarshal 失败）：该 uid 跳过且不进结果（不以零值简报发布），
//     计错误日志供告警；不影响同批其它 uid。
//   - SetInfo 先持久化后写缓存：失败不进缓存，缓存与存储不分叉。
//   - 对外返回快照副本，缓存内对象不被调用方修改。
type InfoMgr struct {
	data *lru.LRUCache

	RedisMgr *redis.RedisMgr
}

func NewInfoMgr() *InfoMgr {
	mgr := new(InfoMgr)
	mgr.data = lru.NewLRUCache(CACHE_SIZE)
	mgr.RedisMgr = redis.NewRedisMgr()
	return mgr
}

func (m *InfoMgr) GetInfo(ctx context.Context, uidList *[]uint64) (*[]*g1_protocol.PbRoleBriefInfo, int) {
	missUid := make([]uint64, 0)
	briefs := make([]*g1_protocol.PbRoleBriefInfo, 0)
	for _, uid := range *uidList {
		v, exist, e := m.data.Get(uid)
		if e != nil {
			return nil, int(g1_protocol.ErrorCode_ERR_FAIL)
		}
		if !exist {
			missUid = append(missUid, uid)
		} else {
			// 快照副本：缓存内对象不因对外返回而被调用方修改。
			briefs = append(briefs, proto.Clone(v.(*g1_protocol.PbRoleBriefInfo)).(*g1_protocol.PbRoleBriefInfo))
		}
	}

	// 如果miss了一部分，则去db拉取；存储错误直接上抛（不得伪装成空列表成功）。
	if len(missUid) > 0 {
		rsp, ret := m.loadBriefFromDB(ctx, missUid)
		if ret != 0 {
			return nil, ret
		}
		if rsp != nil {
			for _, brief := range *rsp {
				briefs = append(briefs, brief)
				_ = m.data.Set(brief.Uid, proto.Clone(brief).(*g1_protocol.PbRoleBriefInfo))
			}
		}
	}

	return &briefs, 0
}

func (m *InfoMgr) SetInfo(ctx context.Context, uid uint64, brief *g1_protocol.PbRoleBriefInfo) int {
	// 先持久化后写缓存（F09）：失败不进缓存，缓存与存储不分叉。
	// 内部约定：0 = 成功（与 GetInfo 一致；协议层按 !=0 判失败）。
	if ret := m.saveBriefToDB(ctx, uid, brief); ret != 0 {
		return ret
	}
	_ = m.data.Set(uid, proto.Clone(brief).(*g1_protocol.PbRoleBriefInfo))
	return 0
}

func (m *InfoMgr) loadBriefFromDB(ctx context.Context, uidList []uint64) (*[]*g1_protocol.PbRoleBriefInfo, int) {
	dbType := uint32(g1_protocol.DBType_DB_TYPE_BRIEF_INFO)
	keys := make([]string, 0, len(uidList))
	for _, v := range uidList {
		key := fmt.Sprintf("%v:%d", g1_protocol.DBType_DB_TYPE_BRIEF_INFO.String(), v)
		keys = append(keys, key)
	}
	rsp, err := m.RedisMgr.MGetBytes(ctx, dbType, keys...)
	if err != nil {
		logger.Errorf("get db brief error | %v", err)
		return nil, int(g1_protocol.ErrorCode_ERR_DB)
	}
	ret := make([]*g1_protocol.PbRoleBriefInfo, 0, len(uidList))
	for _, v := range rsp {
		brief := &g1_protocol.PbRoleBriefInfo{}
		if len(v) == 0 {
			continue // key 不存在：玩家无简报
		}
		// 损坏数据不得以零值简报发布（F09）：跳过并计错误日志。
		if err := proto.Unmarshal(v, brief); err != nil {
			logger.Errorf("corrupt brief in db, skipped {size:%d} | %v", len(v), err)
			continue
		}
		ret = append(ret, brief)
	}
	return &ret, 0
}

func (m *InfoMgr) saveBriefToDB(ctx context.Context, uid uint64, brief *g1_protocol.PbRoleBriefInfo) int {
	dbType := uint32(g1_protocol.DBType_DB_TYPE_BRIEF_INFO)
	key := fmt.Sprintf("%v:%d", g1_protocol.DBType_DB_TYPE_BRIEF_INFO.String(), uid)
	data, err := proto.Marshal(brief)
	if err != nil {
		logger.Errorf("marshal brief error {uid:%d} | %v", uid, err)
		return int(g1_protocol.ErrorCode_ERR_MARSHAL)
	}
	if err := m.RedisMgr.SetBytes(ctx, dbType, key, data, briefCacheTTL()); err != nil {
		logger.Errorf("set db brief error {uid:%d} | %v", uid, err)
		return int(g1_protocol.ErrorCode_ERR_DB)
	}
	return 0
}

// briefCacheTTL 简报缓存 TTL。未配置（0）= 永不过期（现状兼容）。
// 简报由 mainsvr 心跳周期重写，TTL 到期仅造成短暂缺失，下次心跳回填。
func briefCacheTTL() time.Duration {
	days := conf.Get("infosvr.capacity.brief_cache_ttl_days").Int()
	if days <= 0 {
		return 0
	}
	return time.Duration(days) * 24 * time.Hour
}
