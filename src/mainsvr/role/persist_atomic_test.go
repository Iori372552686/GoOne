package role

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	goredis "github.com/redis/go-redis/v9"
)

// capturingRedisClient 捕获 HSet 调用的命令次数与字段集合（F05 验收）。
type capturingRedisClient struct {
	goredis.UniversalClient
	err    error
	hsetCalls int
	hsetFields [][]string // 每次 HSet 命令的字段名集合
}

func (c *capturingRedisClient) HSet(ctx context.Context, _ string, values ...interface{}) *goredis.IntCmd {
	cmd := goredis.NewIntCmd(ctx)
	c.hsetCalls++
	if err := ctx.Err(); err != nil {
		cmd.SetErr(err)
		return cmd
	}
	if c.err != nil {
		cmd.SetErr(c.err)
		return cmd
	}
	if len(values) == 1 {
		if m, ok := values[0].(map[string]interface{}); ok {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			c.hsetFields = append(c.hsetFields, keys)
		}
	}
	cmd.SetVal(int64(len(values)))
	return cmd
}

func withCapturingRedis(client *capturingRedisClient) func() {
	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	rds.RedisMgr.AddClientInstance(uint32(g1_protocol.DBType_DB_TYPE_ROLE), client)
	return func() { rds.RedisMgr = oldMgr }
}

// F05 验收：全量保存是恰好一条多字段 HSET 命令——跨模块提交原子，
// 不存在"余额已写、背包未写"的逐字段中间窗口。
func TestSaveRoleHashSingleCommand(t *testing.T) {
	stub := &capturingRedisClient{}
	restore := withCapturingRedis(stub)
	defer restore()

	role := NewRole(4001)
	if err := saveRoleHash(context.Background(), role, true); err != nil {
		t.Fatalf("全量保存应成功: %v", err)
	}
	if stub.hsetCalls != 1 {
		t.Fatalf("全量保存应恰好 1 条 HSET 命令，实际 %d", stub.hsetCalls)
	}
	if len(stub.hsetFields) != 1 {
		t.Fatalf("应捕获 1 次字段集合，实际 %d", len(stub.hsetFields))
	}
	// 全量写应包含全部 section 字段。
	if got := len(stub.hsetFields[0]); got != len(roleSectionAccessors) {
		t.Fatalf("全量写字段数应 %d，实际 %d", len(roleSectionAccessors), got)
	}
}

// F05 验收：增量保存只提交 dirty 模块命中的字段。
func TestSaveRoleHashIncrementalWritesOnlyDirtySections(t *testing.T) {
	stub := &capturingRedisClient{}
	restore := withCapturingRedis(stub)
	defer restore()

	role := NewRole(4002)
	role.MarkInventoryDirty(1001, false)
	if err := saveRoleHash(context.Background(), role, false); err != nil {
		t.Fatalf("增量保存应成功: %v", err)
	}
	if stub.hsetCalls != 1 {
		t.Fatalf("增量保存应 1 条 HSET，实际 %d", stub.hsetCalls)
	}
	if len(stub.hsetFields) != 1 || len(stub.hsetFields[0]) != 1 || stub.hsetFields[0][0] != "inventory" {
		t.Fatalf("增量保存只应写 inventory 字段，实际 %v", stub.hsetFields)
	}
}

// F05 验收：提交失败时不产生任何已写字段（单命令失败即整体失败），
// 且 dirty mask 保留供重试。
func TestSaveRoleHashFailureWritesNothing(t *testing.T) {
	stub := &capturingRedisClient{err: errors.New("redis unavailable")}
	restore := withCapturingRedis(stub)
	defer restore()

	role := NewRole(4003)
	role.MarkInventoryDirty(1001, false)
	role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_INVENTORY_INFO, "test")

	if err := saveRoleHash(context.Background(), role, false); err == nil {
		t.Fatal("保存失败必须返回错误")
	}
	if len(stub.hsetFields) != 0 {
		t.Fatalf("失败时不应有任何字段写入记录，实际 %v", stub.hsetFields)
	}
	if !hasRoleSection(role.persistDirtyMask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		t.Fatal("失败后 dirty mask 应保留供重试")
	}
}

// 成功路径清 dirty mask（经 SaveToDBSync 入口，mask 清除是调用方契约）。
func TestSaveRoleHashSuccessClearsMask(t *testing.T) {
	stub := &capturingRedisClient{}
	restore := withCapturingRedis(stub)
	defer restore()

	role := NewRole(4004)
	role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_INVENTORY_INFO, "test")
	if err := role.SaveToDBSync(context.Background()); err != nil {
		t.Fatalf("保存应成功: %v", err)
	}
	if hasRoleSection(role.persistDirtyMask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		t.Fatal("成功后对应 dirty 位应清除")
	}
}
