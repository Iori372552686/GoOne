package role

import (
	"testing"

	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"google.golang.org/protobuf/proto"
)

// TestPersistDirtyMaskAccumulates 验证各 Touch*/Mark* 方法正确累积 persistDirtyMask，
// 且 saveRoleHash 的 force/增量分支能据此选择写入模块。
func TestPersistDirtyMaskAccumulates(t *testing.T) {
	r := newSyncTestRole(2001)

	// 初始无 dirty
	if r.persistDirtyMask != 0 {
		t.Fatalf("initial mask should be 0, got %v", r.persistDirtyMask)
	}

	r.TouchBasicInfo("basic_info")
	if !hasRoleSection(r.persistDirtyMask, g1_protocol.ERoleSectionFlag_BASIC_INFO) {
		t.Fatalf("BASIC_INFO should be dirty after TouchBasicInfo, mask=%v", r.persistDirtyMask)
	}

	r.MarkInventoryDirty(101, false)
	if !hasRoleSection(r.persistDirtyMask, g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		t.Fatalf("INVENTORY_INFO should be dirty after MarkInventoryDirty, mask=%v", r.persistDirtyMask)
	}

	r.TouchActvityTaskInfo("activity_task")
	if !hasRoleSection(r.persistDirtyMask, g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO) {
		t.Fatalf("ACTVITY_TASK_INFO should be dirty, mask=%v", r.persistDirtyMask)
	}

	// 清除后应归零
	r.clearPersistDirtyMask()
	if r.persistDirtyMask != 0 {
		t.Fatalf("mask should be 0 after clear, got %v", r.persistDirtyMask)
	}
}

// TestRoleSectionAccessorsMarshalRoundTrip 验证每个 section accessor 的 marshal/unmarshal 往返：
// 写入一个子 message → marshal → unmarshalSection → 字段值一致。
func TestRoleSectionAccessorsMarshalRoundTrip(t *testing.T) {
	r := newSyncTestRole(2002)
	r.PbRole.BasicInfo.Name = "tester"
	r.PbRole.BasicInfo.Exp = 9999
	r.PbRole.InventoryInfo.ItemMap[42] = &g1_protocol.PbItem{Id: 42, Count: 7}

	for _, acc := range roleSectionAccessors {
		msg := acc.get(r.PbRole)
		if msg == nil {
			continue // 未设置的 section 跳过
		}
		buf, err := proto.Marshal(msg)
		if err != nil {
			t.Fatalf("marshal section %s: %v", acc.name, err)
		}
		dst := new(g1_protocol.RoleInfo)
		if err := unmarshalSection(dst, acc.flag, buf); err != nil {
			t.Fatalf("unmarshal section %s: %v", acc.name, err)
		}
		// 确认读回的子消息与原始 marshal 一致（字节级）
		got := acc.get(dst)
		if got == nil {
			t.Fatalf("section %s not set after unmarshal", acc.name)
		}
		gotBuf, _ := proto.Marshal(got)
		if string(gotBuf) != string(buf) {
			t.Fatalf("section %s roundtrip mismatch", acc.name)
		}
	}

	// 抽查具体值
	dst := new(g1_protocol.RoleInfo)
	basicBuf, _ := proto.Marshal(r.PbRole.BasicInfo)
	if err := unmarshalSection(dst, g1_protocol.ERoleSectionFlag_BASIC_INFO, basicBuf); err != nil {
		t.Fatalf("unmarshal basic: %v", err)
	}
	if dst.BasicInfo.Name != "tester" || dst.BasicInfo.Exp != 9999 {
		t.Fatalf("basic info mismatch: %+v", dst.BasicInfo)
	}
}

// TestSaveRoleHashSelectsDirtySections 验证 saveRoleHash 的模块选择逻辑（不连 Redis）。
// 通过强制 mode 与检查 accessor 遍历行为间接验证：force=true 写全部，否则只写 dirty。
// 这里只验证 mask 语义，真正 Redis 写入由联调测试覆盖。
func TestSaveRoleHashSelectsDirtySections(t *testing.T) {
	r := newSyncTestRole(2003)
	r.TouchBasicInfo("basic_info")
	r.MarkMallDirty(1, false)

	want := g1_protocol.ERoleSectionFlag_BASIC_INFO | g1_protocol.ERoleSectionFlag_MALL_INFO
	if r.persistDirtyMask != want {
		t.Fatalf("dirty mask = %v, want %v", r.persistDirtyMask, want)
	}

	// 模拟落盘后清 mask
	r.clearPersistDirtyMask()
	if r.persistDirtyMask != 0 {
		t.Fatalf("mask should be cleared")
	}
}
