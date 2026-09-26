package texas_room

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/db/redis"
	rds "github.com/Iori372552686/GoOne/src/roomcentersvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	goredis "github.com/redis/go-redis/v9"
)

// stubRedisClient 可控失败的 Redis 桩：err 非 nil 时 Set 返回错误；
// 始终尊重传入 ctx 的取消（模拟真实客户端行为，验证 ctx 贯通）。
type stubRedisClient struct {
	goredis.UniversalClient
	err   error
	calls int
}

func (c *stubRedisClient) Set(ctx context.Context, key string, _ interface{}, _ time.Duration) *goredis.StatusCmd {
	cmd := goredis.NewStatusCmd(ctx)
	c.calls++
	if err := ctx.Err(); err != nil {
		cmd.SetErr(err)
		return cmd
	}
	if c.err != nil {
		cmd.SetErr(c.err)
		return cmd
	}
	cmd.SetVal("OK")
	return cmd
}

// Get 供懒恢复路径调用：默认无历史快照（redis.Nil → GetBytes 返回 nil,nil）。
func (c *stubRedisClient) Get(ctx context.Context, _ string) *goredis.StringCmd {
	cmd := goredis.NewStringCmd(ctx)
	if err := ctx.Err(); err != nil {
		cmd.SetErr(err)
		return cmd
	}
	cmd.SetErr(goredis.Nil)
	return cmd
}

func withStubRedis(client *stubRedisClient) func() {
	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	rds.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_TEXAS_ROOM), client)
	return func() { rds.RedisMgr = oldMgr }
}

// F07 验收：Redis 写失败时周期保存返回错误（不再静默吞掉）。
func TestSaveRoomDataToDBPropagatesFailure(t *testing.T) {
	stub := &stubRedisClient{err: errors.New("redis unavailable")}
	restore := withStubRedis(stub)
	defer restore()

	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 1, 9, g1_protocol.RoomStage_LOW, 9999999999) // UpdateRoomInfo 标脏

	if err := mgr.SaveRoomDataToDB(context.Background()); err == nil {
		t.Fatal("保存失败必须聚合返回错误（F07）")
	}
	// 失败保留 dirty，供下一轮重试。
	room := mgr.GetTexasObj(int32(g1_protocol.RoomStage_LOW))
	if !room.CheckChange() {
		t.Fatal("失败后 dirty 应保留")
	}
}

// 成功路径：返回 nil 且清 dirty。
func TestSaveRoomDataToDBSuccessClearsDirty(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 1, 9, g1_protocol.RoomStage_LOW, 9999999999)

	if err := mgr.SaveRoomDataToDB(context.Background()); err != nil {
		t.Fatalf("保存成功应返回 nil: %v", err)
	}
	room := mgr.GetTexasObj(int32(g1_protocol.RoomStage_LOW))
	if room.CheckChange() {
		t.Fatal("成功后 dirty 应清除")
	}
	if stub.calls == 0 {
		t.Fatal("应已触发 Redis 写")
	}
}

// F07 验收：Drain 全量写失败计入 failed（上层据此返回停机错误）。
func TestFlushAllRoomsToDBCountsFailures(t *testing.T) {
	stub := &stubRedisClient{err: errors.New("redis unavailable")}
	restore := withStubRedis(stub)
	defer restore()

	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 1, 9, g1_protocol.RoomStage_HIGH, 9999999999)

	saved, failed := mgr.FlushAllRoomsToDB(context.Background())
	if failed == 0 || saved != 0 {
		t.Fatalf("Redis 失败时应有 failed>0 且 saved=0，实际 saved=%d failed=%d", saved, failed)
	}
}

// F07 验收：取消的 ctx 能传导到底层 Redis 调用并快速失败（排空预算被尊重）。
func TestSaveRoomDataToDBRespectsCancelledContext(t *testing.T) {
	stub := &stubRedisClient{}
	restore := withStubRedis(stub)
	defer restore()

	mgr, _ := newTestMgr(1)
	seedRoom(t, mgr, 100, 1, 9, g1_protocol.RoomStage_LOW, 9999999999)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mgr.SaveRoomDataToDB(ctx)
	if err == nil {
		t.Fatal("取消的 ctx 应使保存快速失败")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("错误应包含 context.Canceled，实际: %v", err)
	}
}
