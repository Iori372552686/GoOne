package service

import (
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
)

// TestProtoMarshalRoundTrip 验证 mysqlsvr 使用的核心 proto 消息的 marshal/unmarshal 往返。
// mysqlsvr 的 handler 依赖 OrmMgr/DB 全局单例，核心路径无法纯单测；
// 这里覆盖协议层的数据序列化正确性（saveRoomInfo/saveGameInfo 的前置条件）。
func TestProtoMarshalRoundTrip(t *testing.T) {
	t.Run("MysqlTexasRoomInfo", func(t *testing.T) {
		orig := &g1_protocol.MysqlTexasRoomInfo{
			RoomId:     12345,
			TableId:    67890,
			UpdateTime: 100,
		}
		buf, err := proto.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := &g1_protocol.MysqlTexasRoomInfo{}
		if err := proto.Unmarshal(buf, got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.RoomId != 12345 || got.TableId != 67890 || got.UpdateTime != 100 {
			t.Fatalf("roundtrip mismatch: %+v", got)
		}
	})

	t.Run("MysqlTexasGameInfo", func(t *testing.T) {
		orig := &g1_protocol.MysqlTexasGameInfo{GameId: "game-001"}
		buf, err := proto.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := &g1_protocol.MysqlTexasGameInfo{}
		if err := proto.Unmarshal(buf, got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.GameId != "game-001" {
			t.Fatalf("GameId mismatch: %s", got.GameId)
		}
	})

	t.Run("MysqlTexasPlayerInfo", func(t *testing.T) {
		orig := &g1_protocol.MysqlTexasPlayerInfo{Uid: 99999, TableId: 888}
		buf, err := proto.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := &g1_protocol.MysqlTexasPlayerInfo{}
		if err := proto.Unmarshal(buf, got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Uid != 99999 || got.TableId != 888 {
			t.Fatalf("roundtrip mismatch: %+v", got)
		}
	})
}

// TestProtoUnmarshalNilBuf 验证 proto.Unmarshal 对 nil buf 的容错（返回零值不 panic）。
// saveRoomInfo 等函数内部对 nil/空 buf 走这个路径。
func TestProtoUnmarshalNilBuf(t *testing.T) {
	got := &g1_protocol.MysqlTexasRoomInfo{}
	if err := proto.Unmarshal(nil, got); err != nil {
		t.Fatalf("unmarshal nil should not error: %v", err)
	}
	if got.RoomId != 0 {
		t.Fatalf("nil unmarshal should yield zero value, got RoomId=%d", got.RoomId)
	}
}
