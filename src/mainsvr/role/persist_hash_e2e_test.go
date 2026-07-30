package role

import (
	"os"
	"strconv"
	"testing"

	"github.com/Iori372552686/GoOne/lib/db/redis"
	rds "github.com/Iori372552686/GoOne/src/mainsvr/globals/rds"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
)

// 端到端联调：连真实 Redis 验证角色 hash 持久化。
// 触发：GOONE_REDIS_ADDR=<host:port> GOONE_REDIS_PASS=<pass> go test -run TestRoleHashE2E ./src/mainsvr/role/

func redisAddr() (addr, pass string) {
	return os.Getenv("GOONE_REDIS_ADDR"), os.Getenv("GOONE_REDIS_PASS")
}

func initE2ERedis(t *testing.T) (cleanup func()) {
	t.Helper()
	addr, pass := redisAddr()
	if addr == "" {
		t.Skip("GOONE_REDIS_ADDR 未设置，跳过真实 Redis 联调")
	}

	oldMgr := rds.RedisMgr
	rds.RedisMgr = redis.NewRedisMgr()
	host, port := splitHostPortE2E(addr)
	if err := rds.RedisMgr.AddInstance(1, host, port, pass, 0, false); err != nil {
		t.Fatalf("redis AddInstance err: %v", err)
	}
	return func() { rds.RedisMgr = oldMgr }
}

func splitHostPortE2E(addr string) (string, int) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			p, _ := strconv.Atoi(addr[i+1:])
			return addr[:i], p
		}
	}
	return addr, 6379
}

func roleKeyE2E(uid uint64) string {
	return g1_protocol.DBType_DB_TYPE_ROLE.String() + ":" + strconv.FormatUint(uid, 10)
}

// TestRoleHashE2EFullThenIncremental 端到端验证角色 hash 持久化：
//  1. 首次全量写（force）→ hash 里有所有 field
//  2. 增量改 basic → 只更新 basic field，其它 field 不变
//  3. loadRoleHash 读回 → 数据与内存一致
func TestRoleHashE2EFullThenIncremental(t *testing.T) {
	cleanup := initE2ERedis(t)
	defer cleanup()

	uid := uint64(990001)
	key := roleKeyE2E(uid)
	instID := uint32(g1_protocol.DBType_DB_TYPE_ROLE)
	rds.RedisMgr.DelKey(instID, key)

	// 1) 构造角色并全量写
	r := NewRole(uid)
	r.PbRole.BasicInfo.Name = "tester_e2e"
	r.PbRole.BasicInfo.Exp = 88888
	r.PbRole.InventoryInfo.ItemMap[1001] = &g1_protocol.PbItem{Id: 1001, Count: 5}
	r.PbRole.GameInfo.PlayRoomIds = []uint64{100, 200, 300}

	t.Run("FirstFullWrite", func(t *testing.T) {
		if err := r.SaveToDBSync(); err != nil {
			t.Fatalf("SaveToDBSync: %v", err)
		}
		var fields map[string][]byte
		if err := rds.RedisMgr.DoFlatCmd(instID, &fields, "HGETALL", key); err != nil {
			t.Fatalf("HGETALL: %v", err)
		}
		t.Logf("hash fields after full write: %d", len(fields))
		if len(fields) < 10 {
			t.Fatalf("full write should produce many fields, got %d", len(fields))
		}
	})

	t.Run("IncrementalWriteBasicOnly", func(t *testing.T) {
		var invBefore []byte
		rds.RedisMgr.DoFlatCmd(instID, &invBefore, "HGET", key, "inventory")

		r.TouchBasicInfo("test_modify")
		r.PbRole.BasicInfo.Exp = 99999
		if err := saveRoleHash(r, false); err != nil {
			t.Fatalf("incremental saveRoleHash: %v", err)
		}
		r.clearPersistDirtyMask()

		var basicAfter []byte
		rds.RedisMgr.DoFlatCmd(instID, &basicAfter, "HGET", key, "basic")
		if len(basicAfter) == 0 {
			t.Fatal("basic field missing after incremental write")
		}

		var invAfter []byte
		rds.RedisMgr.DoFlatCmd(instID, &invAfter, "HGET", key, "inventory")
		if string(invAfter) != string(invBefore) {
			t.Fatal("inventory field changed during basic-only incremental write")
		}
		t.Log("incremental write correctly left inventory field untouched")
	})

	t.Run("LoadBackConsistency", func(t *testing.T) {
		info, err := loadRoleHash(uid)
		if err != nil {
			t.Fatalf("loadRoleHash: %v", err)
		}
		if info == nil {
			t.Fatal("loadRoleHash returned nil")
		}
		if info.BasicInfo.Name != "tester_e2e" {
			t.Errorf("Name = %q, want tester_e2e", info.BasicInfo.Name)
		}
		if info.BasicInfo.Exp != 99999 {
			t.Errorf("Exp = %d, want 99999", info.BasicInfo.Exp)
		}
		if got := info.InventoryInfo.ItemMap[1001].GetCount(); got != 5 {
			t.Errorf("ItemMap[1001].Count = %d, want 5", got)
		}
		if len(info.GameInfo.PlayRoomIds) != 3 || info.GameInfo.PlayRoomIds[0] != 100 {
			t.Errorf("PlayRoomIds = %v, want [100 200 300]", info.GameInfo.PlayRoomIds)
		}
		t.Log("load back data consistent with memory")
	})

	rds.RedisMgr.DelKey(instID, key)
}
