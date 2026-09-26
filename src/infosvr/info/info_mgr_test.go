package info

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/db/redis"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/golang/protobuf/proto"
	goredis "github.com/redis/go-redis/v9"
)

// stubRedisClient 可控的 Redis 桩：MGet 返回预置数据或错误，Set 可控失败。
type stubRedisClient struct {
	goredis.UniversalClient
	mgetErr  error
	mgetVals map[string][]byte
	setErr   error
	setCalls int
}

func (c *stubRedisClient) MGet(ctx context.Context, keys ...string) *goredis.SliceCmd {
	cmd := goredis.NewSliceCmd(ctx)
	if c.mgetErr != nil {
		cmd.SetErr(c.mgetErr)
		return cmd
	}
	vals := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		if v, ok := c.mgetVals[k]; ok {
			vals = append(vals, string(v))
		} else {
			vals = append(vals, nil)
		}
	}
	cmd.SetVal(vals)
	return cmd
}

func (c *stubRedisClient) Set(ctx context.Context, key string, _ interface{}, _ time.Duration) *goredis.StatusCmd {
	cmd := goredis.NewStatusCmd(ctx)
	c.setCalls++
	if c.setErr != nil {
		cmd.SetErr(c.setErr)
		return cmd
	}
	cmd.SetVal("OK")
	return cmd
}

func newTestMgr(stub *stubRedisClient) *InfoMgr {
	mgr := NewInfoMgr()
	mgr.RedisMgr = redis.NewRedisMgr()
	mgr.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_BRIEF_INFO), stub)
	return mgr
}

func briefKey(uid uint64) string {
	return g1_protocol.DBType_DB_TYPE_BRIEF_INFO.String() + ":" + strconv.FormatUint(uid, 10)
}

func mustMarshal(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// F09 验收：存储不可用返回 ERR_DB，不再伪装成"成功的空列表"。
func TestGetInfoStorageFailureReturnsDBError(t *testing.T) {
	mgr := newTestMgr(&stubRedisClient{mgetErr: errors.New("redis unavailable")})

	res, ret := mgr.GetInfo(context.Background(), &[]uint64{5001})
	if ret != int(g1_protocol.ErrorCode_ERR_DB) {
		t.Fatalf("存储故障应返回 ERR_DB，实际 %d", ret)
	}
	if res != nil {
		t.Fatalf("失败时不应返回结果列表，实际 %v", res)
	}
}

// F09 验收：损坏数据（unmarshal 失败）不进入成功结果；同批其它 uid 不受影响。
func TestGetInfoSkipsCorruptEntries(t *testing.T) {
	stub := &stubRedisClient{mgetVals: map[string][]byte{
		briefKey(5001): mustMarshal(t, &g1_protocol.PbRoleBriefInfo{Uid: 5001, Name: "ok"}),
		briefKey(5002): []byte("not-a-valid-protobuf"),
	}}
	mgr := newTestMgr(stub)

	res, ret := mgr.GetInfo(context.Background(), &[]uint64{5001, 5002})
	if ret != 0 {
		t.Fatalf("部分损坏不应整体失败，实际 ret=%d", ret)
	}
	if len(*res) != 1 || (*res)[0].Uid != 5001 {
		t.Fatalf("损坏条目应被跳过，只返回有效项，实际 %v", *res)
	}
}

// F09 验收：缓存命中不触发存储；对外返回快照副本（修改返回值不影响缓存）。
func TestGetInfoReturnsSnapshotCopies(t *testing.T) {
	stub := &stubRedisClient{mgetVals: map[string][]byte{
		briefKey(5003): mustMarshal(t, &g1_protocol.PbRoleBriefInfo{Uid: 5003, Name: "orig"}),
	}}
	mgr := newTestMgr(stub)

	// 首次拉取入缓存。
	res, ret := mgr.GetInfo(context.Background(), &[]uint64{5003})
	if ret != 0 || len(*res) != 1 {
		t.Fatalf("首次拉取应成功: ret=%d res=%v", ret, res)
	}

	// 修改返回的快照。
	(*res)[0].Name = "mutated-by-caller"

	// 再次读取（应命中缓存）：值不受调用方修改影响。
	res2, ret := mgr.GetInfo(context.Background(), &[]uint64{5003})
	if ret != 0 || len(*res2) != 1 {
		t.Fatalf("缓存命中应成功: ret=%d", ret)
	}
	if (*res2)[0].Name != "orig" {
		t.Fatalf("缓存应返回未被调用方修改的快照，实际 %q", (*res2)[0].Name)
	}
}

// F09 验收：SetInfo 先持久化后缓存——持久化失败不进缓存。
func TestSetInfoPersistsBeforeCaching(t *testing.T) {
	stub := &stubRedisClient{setErr: errors.New("redis unavailable")}
	mgr := newTestMgr(stub)

	brief := &g1_protocol.PbRoleBriefInfo{Uid: 5004, Name: "x"}
	if ret := mgr.SetInfo(context.Background(), 5004, brief); ret != int(g1_protocol.ErrorCode_ERR_DB) {
		t.Fatalf("持久化失败应返回 ERR_DB，实际 %d", ret)
	}
	// 失败后不应入缓存：换成成功桩后仍需从 db 拉取。
	stub.setErr = nil
	if ret := mgr.SetInfo(context.Background(), 5004, brief); ret != 0 {
		t.Fatalf("重试应成功，实际 %d", ret)
	}
	if stub.setCalls != 2 {
		t.Fatalf("应有两次持久化尝试，实际 %d", stub.setCalls)
	}

	// 成功后缓存生效：再读走缓存（mget 配置为错误也不会被触发）。
	stub.mgetErr = errors.New("should-not-touch-db")
	res, ret := mgr.GetInfo(context.Background(), &[]uint64{5004})
	if ret != 0 || len(*res) != 1 || (*res)[0].Uid != 5004 {
		t.Fatalf("成功写入后应命中缓存: ret=%d res=%v", ret, res)
	}
}
