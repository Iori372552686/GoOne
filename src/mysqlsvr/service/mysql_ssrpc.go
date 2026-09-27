package service

import (
	"context"
	"errors"

	"github.com/Iori372552686/GoOne/api/gen/game/mysqlsvr/v1"
	"github.com/Iori372552686/GoOne/lib/api/gerr"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/repository"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

type MysqlServiceImpl struct {
	mysqlsvrv1.MysqlServiceSS
	repo repository.Store
}

func NewMysqlServiceImpl(repo repository.Store) *MysqlServiceImpl {
	return &MysqlServiceImpl{repo: repo}
}

func (s *MysqlServiceImpl) UpdateRoleInfo(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerUpdateRoleInfoReq) (*g1_protocol.MysqlInnerUpdateRoleInfoRsp, error) {
	rsp := &g1_protocol.MysqlInnerUpdateRoleInfoRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if ctx == nil || s.repo == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}
	if err := s.repo.UpdateRole(requestContext(ctx), ctx.Uid(), req.GetName()); err != nil {
		logger.Errorf("failed to update role info | %v", err)
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "update_role", err)
	}
	return rsp, nil
}

func (s *MysqlServiceImpl) SearchRole(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerSearchRoleReq) (*g1_protocol.MysqlInnerSearchRoleRsp, error) {
	rsp := &g1_protocol.MysqlInnerSearchRoleRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if s.repo == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}
	uid, err := s.repo.SearchRole(requestContext(ctx), req.GetSearchString())
	if err != nil {
		logger.Errorf("failed to select role info: %v", err)
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "search_role", err)
	}
	rsp.Uid = uid
	return rsp, nil
}

// SaveRoleData 保存角色全量快照（L3）。同步 UPSERT（毫秒级、幂等），不进异步池：
// ack 语义清晰，运行期的异步性由 mainsvr 侧 one-way 投递提供。
// 陈旧写被拒（ErrStaleUpdate）视为正常——更新的快照已落库，返回 OK。
func (s *MysqlServiceImpl) SaveRoleData(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerSaveRoleDataReq) (*g1_protocol.MysqlInnerSaveRoleDataRsp, error) {
	rsp := &g1_protocol.MysqlInnerSaveRoleDataRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if s.repo == nil {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_INTERNAL, "biz_error", "")
	}
	uid := req.GetUid()
	if uid == 0 && ctx != nil {
		uid = ctx.Uid()
	}
	if uid == 0 {
		return rsp, gerr.New(g1_protocol.ErrorCode_ERR_MARSHAL, "save_role_data: missing uid", "")
	}
	if err := s.repo.UpsertRoleData(requestContext(ctx), uid, req.GetData(), req.GetUpdateTime()); err != nil {
		if errors.Is(err, repository.ErrStaleUpdate) {
			logger.Warningf("mysqlsvr rejected stale role data snapshot | %v", err)
			return rsp, nil
		}
		logger.Errorf("failed to save role data {uid:%d} | %v", uid, err)
		return rsp, gerr.Wrap(g1_protocol.ErrorCode_ERR_FAIL, "save_role_data", err)
	}
	return rsp, nil
}

// LoadRoleData 按 uid 读取角色全量快照（L2 miss 回源）。miss 时 data 为空、ret 为 OK。
func (s *MysqlServiceImpl) LoadRoleData(ctx *ssrpc.Context, req *g1_protocol.MysqlInnerLoadRoleDataReq) (*g1_protocol.MysqlInnerLoadRoleDataRsp, error) {
	if s.repo == nil {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_INTERNAL, "database repository unavailable")
	}
	uid := req.GetUid()
	if uid == 0 && ctx != nil {
		uid = ctx.Uid()
	}
	if uid == 0 {
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_MARSHAL, "load_role_data: missing uid")
	}
	item, err := s.repo.LoadRoleData(requestContext(ctx), uid)
	if err != nil {
		logger.Errorf("failed to load role data {uid:%d} | %v", uid, err)
		return nil, ssrpc.E(g1_protocol.ErrorCode_ERR_DB, "load role data failed")
	}
	rsp := &g1_protocol.MysqlInnerLoadRoleDataRsp{Ret: &g1_protocol.Ret{Code: g1_protocol.ErrorCode_ERR_OK}}
	if item != nil {
		rsp.Data = item.Data
		rsp.UpdateTime = item.UpdateTime
	}
	return rsp, nil
}

func requestContext(ctx *ssrpc.Context) context.Context {
	if ctx == nil || ctx.Context == nil {
		return context.Background()
	}
	return ctx.Context
}
