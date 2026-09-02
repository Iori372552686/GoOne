package service

import (
	"context"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	"gorm.io/gorm"
)

type contextResult struct {
	err         error
	hasDeadline bool
}

type fakeStore struct {
	saveRoom func(context.Context, *g1_protocol.MysqlTexasRoomInfo) error
}

func (f *fakeStore) UpdateRole(context.Context, uint64, string) error   { return nil }
func (f *fakeStore) SearchRole(context.Context, string) (uint64, error) { return 0, nil }
func (f *fakeStore) QueryRoom(context.Context, *g1_protocol.QueryRoomInfoReq) ([]*g1_protocol.MysqlTexasRoomInfo, error) {
	return nil, nil
}
func (f *fakeStore) QueryPlayer(context.Context, *g1_protocol.QueryPlayerInfoReq) ([]*g1_protocol.MysqlTexasPlayerInfo, error) {
	return nil, nil
}
func (f *fakeStore) GetGame(context.Context, string) (*g1_protocol.MysqlTexasGameInfo, error) {
	return nil, gorm.ErrRecordNotFound
}
func (f *fakeStore) SaveRoom(ctx context.Context, item *g1_protocol.MysqlTexasRoomInfo) error {
	return f.saveRoom(ctx, item)
}
func (f *fakeStore) SaveGame(context.Context, *g1_protocol.MysqlTexasGameInfo) error { return nil }
func (f *fakeStore) InsertPlayer(context.Context, *g1_protocol.MysqlTexasPlayerInfo) error {
	return nil
}

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

func TestAsyncWriteDetachesRequestCancellationAndAddsDeadline(t *testing.T) {
	manager.Start()
	t.Cleanup(manager.Close)
	resultCh := make(chan contextResult, 1)
	store := &fakeStore{saveRoom: func(ctx context.Context, _ *g1_protocol.MysqlTexasRoomInfo) error {
		_, hasDeadline := ctx.Deadline()
		resultCh <- contextResult{err: ctx.Err(), hasDeadline: hasDeadline}
		return nil
	}}
	service := NewMysqlServiceImpl(store)
	data, err := proto.Marshal(&g1_protocol.MysqlTexasRoomInfo{RoomId: 10, TableId: 20})
	if err != nil {
		t.Fatal(err)
	}
	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.Update(&ssrpc.Context{Context: requestCtx}, &g1_protocol.MysqlInnerUpdateReq{
		Id: 1, DataType: g1_protocol.DataType_DATA_TYPE_TEXAS_ROOM_INFO, Data: data,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("async context inherited request cancellation: %v", result.err)
		}
		if !result.hasDeadline {
			t.Fatal("async context has no fixed write deadline")
		}
	case <-time.After(time.Second):
		t.Fatal("async repository call did not run")
	}
}
