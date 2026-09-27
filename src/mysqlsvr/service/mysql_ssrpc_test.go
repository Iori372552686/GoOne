package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/repository"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

type fakeStore struct {
	upsertRole func(context.Context, uint64, []byte, int64) error
	loadRole   func(context.Context, uint64) (*g1_protocol.MysqlRoleData, error)
}

func (f *fakeStore) UpdateRole(context.Context, uint64, string) error    { return nil }
func (f *fakeStore) SearchRole(context.Context, string) (uint64, error) { return 0, nil }
func (f *fakeStore) UpsertRoleData(ctx context.Context, uid uint64, data []byte, ts int64) error {
	if f.upsertRole == nil {
		return nil
	}
	return f.upsertRole(ctx, uid, data, ts)
}
func (f *fakeStore) LoadRoleData(ctx context.Context, uid uint64) (*g1_protocol.MysqlRoleData, error) {
	if f.loadRole == nil {
		return nil, nil
	}
	return f.loadRole(ctx, uid)
}

// SaveRoleData 成功路径：落库成功返回 OK。
func TestSaveRoleDataHandlerSuccess(t *testing.T) {
	var gotUID uint64
	var gotData []byte
	svc := NewMysqlServiceImpl(&fakeStore{upsertRole: func(_ context.Context, uid uint64, data []byte, _ int64) error {
		gotUID, gotData = uid, data
		return nil
	}})
	rsp, err := svc.SaveRoleData(&ssrpc.Context{Context: context.Background()},
		&g1_protocol.MysqlInnerSaveRoleDataReq{Uid: 7, Data: []byte("snap"), UpdateTime: 100})
	if err != nil {
		t.Fatalf("SaveRoleData() error = %v", err)
	}
	if rsp == nil || rsp.Ret == nil || rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("SaveRoleData() rsp = %+v, want ERR_OK", rsp)
	}
	if gotUID != 7 || string(gotData) != "snap" {
		t.Fatalf("repo got uid=%d data=%q", gotUID, gotData)
	}
}

// 陈旧写被持久层拒绝时，handler 视为正常（更新的快照已落库）。
func TestSaveRoleDataStaleRejectedReturnsOK(t *testing.T) {
	svc := NewMysqlServiceImpl(&fakeStore{upsertRole: func(context.Context, uint64, []byte, int64) error {
		return repository.ErrStaleUpdate
	}})
	rsp, err := svc.SaveRoleData(&ssrpc.Context{Context: context.Background()},
		&g1_protocol.MysqlInnerSaveRoleDataReq{Uid: 7, Data: []byte("old"), UpdateTime: 1})
	if err != nil {
		t.Fatalf("陈旧写应返回 OK 而非错误: %v", err)
	}
	if rsp == nil || rsp.Ret == nil || rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("陈旧写 rsp = %+v, want ERR_OK", rsp)
	}
}

// 持久层真实故障必须向上传播（mainsvr 侧保留重试标记的依据）。
func TestSaveRoleDataRepoErrorPropagates(t *testing.T) {
	svc := NewMysqlServiceImpl(&fakeStore{upsertRole: func(context.Context, uint64, []byte, int64) error {
		return errors.New("db down")
	}})
	if _, err := svc.SaveRoleData(&ssrpc.Context{Context: context.Background()},
		&g1_protocol.MysqlInnerSaveRoleDataReq{Uid: 7, Data: []byte("x"), UpdateTime: 1}); err == nil {
		t.Fatal("持久层故障必须传播错误")
	}
}

// 缺 uid（req 与 ctx 都为 0）必须拒绝，不得写 uid=0 行。
func TestSaveRoleDataMissingUIDRejected(t *testing.T) {
	svc := NewMysqlServiceImpl(&fakeStore{})
	if _, err := svc.SaveRoleData(nil, &g1_protocol.MysqlInnerSaveRoleDataReq{Data: []byte("x")}); err == nil {
		t.Fatal("uid=0 且无事务上下文时必须拒绝")
	}
}

// LoadRoleData 命中：data 原样返回；miss：空 data + OK。
func TestLoadRoleDataHandler(t *testing.T) {
	svc := NewMysqlServiceImpl(&fakeStore{loadRole: func(_ context.Context, uid uint64) (*g1_protocol.MysqlRoleData, error) {
		if uid == 9 {
			return &g1_protocol.MysqlRoleData{Uid: 9, Data: []byte("blob"), UpdateTime: 42}, nil
		}
		return nil, nil
	}})

	rsp, err := svc.LoadRoleData(&ssrpc.Context{Context: context.Background()},
		&g1_protocol.MysqlInnerLoadRoleDataReq{Uid: 9})
	if err != nil {
		t.Fatalf("LoadRoleData() error = %v", err)
	}
	if string(rsp.Data) != "blob" || rsp.UpdateTime != 42 {
		t.Fatalf("LoadRoleData() = %q/%d, want blob/42", rsp.Data, rsp.UpdateTime)
	}

	rsp, err = svc.LoadRoleData(&ssrpc.Context{Context: context.Background()},
		&g1_protocol.MysqlInnerLoadRoleDataReq{Uid: 10})
	if err != nil {
		t.Fatalf("miss LoadRoleData() error = %v", err)
	}
	if len(rsp.Data) != 0 {
		t.Fatalf("miss 应返回空 data, got %q", rsp.Data)
	}
}
