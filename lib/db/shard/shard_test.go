package shard

import "testing"

// 契约用例：分片公式一旦发布不可变更。以下期望值由当前实现生成后固定，
// 若改动导致期望值变化，必须按"公式变更 = 数据迁移"流程评审，不得静默更新。
func TestResolve_ContractValues(t *testing.T) {
	rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: 16}

	cases := []struct {
		uid  uint64
		want string // 期望物理表名
	}{
		{0, "role_data_5"},
		{1, "role_data_4"},
		{100, "role_data_1"},
		{9527, "role_data_1"},
		{1 << 40, "role_data_10"},
	}
	for _, c := range cases {
		got, err := rule.Resolve(RouteParams{Uid: c.uid})
		if err != nil {
			t.Fatalf("resolve uid=%d: %v", c.uid, err)
		}
		if got.Table != c.want {
			t.Errorf("uid=%d: table=%q, want %q", c.uid, got.Table, c.want)
		}
	}
}

func TestResolve_NoShards(t *testing.T) {
	for _, shards := range []int{0, 1} {
		rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: shards}
		got, err := rule.Resolve(RouteParams{Uid: 9527})
		if err != nil {
			t.Fatalf("shards=%d: %v", shards, err)
		}
		if got.Table != "role_data" {
			t.Errorf("shards=%d: table=%q, want role_data（无后缀）", shards, got.Table)
		}
	}
}

func TestResolve_DefaultInstance(t *testing.T) {
	rule := &Rule{Name: "role_data", TableBase: "role_data"}
	got, err := rule.Resolve(RouteParams{Uid: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Instance != "default" {
		t.Errorf("instance=%q, want default", got.Instance)
	}

	rule.Instance = "game_db_2"
	got, err = rule.Resolve(RouteParams{Uid: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Instance != "game_db_2" {
		t.Errorf("instance=%q, want game_db_2", got.Instance)
	}
}

func TestResolve_Stability(t *testing.T) {
	rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: 64}
	first, err := rule.Resolve(RouteParams{Uid: 123456789})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		got, err := rule.Resolve(RouteParams{Uid: 123456789, Zone: "z1", Env: "dev"})
		if err != nil {
			t.Fatal(err)
		}
		if got != first {
			t.Fatalf("route unstable for same uid: %+v vs %+v", got, first)
		}
	}
}

// 分布均衡性：顺序 uid 在 16 表上的落点不应集中（每桶占比偏离均值不超过 1/3）。
func TestResolve_Distribution(t *testing.T) {
	const shards = 16
	const samples = 16000

	counts := make(map[uint32]int, shards)
	for uid := uint64(1); uid <= samples; uid++ {
		table := fnv32a(uid) % shards
		counts[table]++
	}

	want := samples / shards
	tolerance := want / 3
	for i := 0; i < shards; i++ {
		c := counts[uint32(i)]
		if diff := c - want; diff > tolerance || -diff > tolerance {
			t.Errorf("bucket %d: count=%d, want %d±%d（fnv32a 低比特分布退化？）", i, c, want, tolerance)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		rule Rule
		want bool // 是否应通过
	}{
		{Rule{Name: "r", TableBase: "t", TableShards: 1}, true},
		{Rule{Name: "r", TableBase: "t", TableShards: 0}, true},
		{Rule{Name: "r", TableBase: "t", TableShards: 64}, true},
		{Rule{Name: "", TableBase: "t"}, false},
		{Rule{Name: "r", TableBase: ""}, false},
		{Rule{Name: "r", TableBase: "t", TableShards: -1}, false},
	}
	for i, c := range cases {
		err := c.rule.Validate()
		if (err == nil) != c.want {
			t.Errorf("case %d: err=%v, wantPass=%v", i, err, c.want)
		}
	}
}
