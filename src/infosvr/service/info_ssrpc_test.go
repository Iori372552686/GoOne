package service

import (
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// TestInfoProtoMarshalRoundTrip 验证 infosvr 使用的核心 proto 消息的 marshal/unmarshal 往返。
// infosvr handler 依赖 InfoMgr（Redis LRU 全局单例），核心路径无法纯单测；
// 这里覆盖协议层数据序列化正确性。
func TestInfoProtoMarshalRoundTrip(t *testing.T) {
	t.Run("InfoGetBriefInfoReq", func(t *testing.T) {
		orig := &g1_protocol.InfoGetBriefInfoReq{UidList: []uint64{1001, 1002, 1003}}
		buf, err := proto.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := &g1_protocol.InfoGetBriefInfoReq{}
		if err := proto.Unmarshal(buf, got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(got.UidList) != 3 || got.UidList[0] != 1001 {
			t.Fatalf("UidList mismatch: %v", got.UidList)
		}
	})

	t.Run("PbRoleBriefInfo", func(t *testing.T) {
		orig := &g1_protocol.PbRoleBriefInfo{Uid: 5001, Name: "alice", Level: 30}
		buf, err := proto.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := &g1_protocol.PbRoleBriefInfo{}
		if err := proto.Unmarshal(buf, got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Uid != 5001 || got.Name != "alice" || got.Level != 30 {
			t.Fatalf("roundtrip mismatch: %+v", got)
		}
	})

	t.Run("InfoGetIconDescReq", func(t *testing.T) {
		orig := &g1_protocol.InfoGetIconDescReq{UidList: []uint64{2001}}
		buf, err := proto.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := &g1_protocol.InfoGetIconDescReq{}
		if err := proto.Unmarshal(buf, got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(got.UidList) != 1 || got.UidList[0] != 2001 {
			t.Fatalf("UidList mismatch: %v", got.UidList)
		}
	})
}

// TestInfoRetCreation 验证响应消息的 Ret 字段初始化与 gerr 设码的兼容性
// （框架 ApplyErrCode 依赖 Ret 字段存在）。
func TestInfoRetCreation(t *testing.T) {
	rsp := &g1_protocol.InfoGetBriefInfoRsp{}
	if rsp.Ret != nil {
		t.Fatal("Ret should be nil before init")
	}
	// 模拟框架 ApplyErrCode 自动创建 Ret 并设码
	rsp.Ret = &g1_protocol.Ret{}
	rsp.Ret.Code = g1_protocol.ErrorCode_ERR_OK
	if rsp.Ret.Code != g1_protocol.ErrorCode_ERR_OK {
		t.Fatalf("Ret.Code = %v, want ERR_OK", rsp.Ret.Code)
	}
}
