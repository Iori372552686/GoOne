package role

import (
	"testing"

	"github.com/Iori372552686/GoOne/module/conf"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func newSyncTestRole(uid uint64) *Role {
	r := NewRole(uid)
	r.PbRole.ConnSvrInfo.BusId = 1
	return r
}

func TestPrepareSyncPayloadBuildsInventoryPatch(t *testing.T) {
	r := newSyncTestRole(1001)
	r.PbRole.InventoryInfo.ItemMap[101] = &g1_protocol.PbItem{Id: 101, Count: 3}
	r.MarkInventoryDirty(101, false)
	r.MarkInventoryDirty(202, true)

	fullMask, requestedPatchMask, actualPatchMask, legacy, v2 := r.prepareSyncPayload(true)
	if fullMask != 0 {
		t.Fatalf("unexpected full mask: %v", fullMask)
	}
	if requestedPatchMask != g1_protocol.ERoleSectionFlag_INVENTORY_INFO {
		t.Fatalf("unexpected requested patch mask: %v", requestedPatchMask)
	}
	if actualPatchMask != g1_protocol.ERoleSectionFlag_INVENTORY_INFO {
		t.Fatalf("unexpected actual patch mask: %v", actualPatchMask)
	}
	if legacy != nil {
		t.Fatalf("legacy payload should be nil when patch sync is enabled")
	}
	if v2 == nil || v2.InventoryPatch == nil {
		t.Fatalf("expected inventory patch payload")
	}
	if len(v2.InventoryPatch.UpsertItems) != 1 || v2.InventoryPatch.UpsertItems[0].GetId() != 101 {
		t.Fatalf("unexpected inventory upserts: %+v", v2.InventoryPatch.UpsertItems)
	}
	if len(v2.InventoryPatch.DeleteItemIds) != 1 || v2.InventoryPatch.DeleteItemIds[0] != 202 {
		t.Fatalf("unexpected inventory deletes: %+v", v2.InventoryPatch.DeleteItemIds)
	}
}

func TestPrepareSyncPayloadDoesNotDuplicateFullAndPatchSections(t *testing.T) {
	r := newSyncTestRole(1002)
	r.PbRole.InventoryInfo.ItemMap[101] = &g1_protocol.PbItem{Id: 101, Count: 5}
	r.MarkInventoryDirty(101, false)
	r.MarkFullSync(g1_protocol.ERoleSectionFlag_INVENTORY_INFO)

	fullMask, requestedPatchMask, actualPatchMask, legacy, v2 := r.prepareSyncPayload(true)
	if legacy != nil {
		t.Fatalf("legacy payload should be nil when patch sync is enabled")
	}
	if fullMask != g1_protocol.ERoleSectionFlag_INVENTORY_INFO {
		t.Fatalf("unexpected full mask: %v", fullMask)
	}
	if requestedPatchMask != 0 || actualPatchMask != 0 {
		t.Fatalf("inventory section should not be sent as both full and patch")
	}
	if v2 == nil || v2.RoleInfo == nil || v2.RoleInfo.InventoryInfo == nil {
		t.Fatalf("expected full inventory role_info payload")
	}
	if v2.InventoryPatch != nil {
		t.Fatalf("inventory patch should be nil when full section is present")
	}
}

func TestInventoryDirtySetKeepsNetChange(t *testing.T) {
	r1 := newSyncTestRole(1003)
	r1.MarkInventoryDirty(101, false)
	r1.MarkInventoryDirty(101, true)

	_, _, actualPatchMask, _, v2 := r1.prepareSyncPayload(true)
	if actualPatchMask != g1_protocol.ERoleSectionFlag_INVENTORY_INFO {
		t.Fatalf("expected inventory patch mask, got %v", actualPatchMask)
	}
	if v2 == nil || v2.InventoryPatch == nil {
		t.Fatalf("expected inventory patch payload")
	}
	if len(v2.InventoryPatch.UpsertItems) != 0 {
		t.Fatalf("upsert should be dropped after delete, got %+v", v2.InventoryPatch.UpsertItems)
	}
	if len(v2.InventoryPatch.DeleteItemIds) != 1 || v2.InventoryPatch.DeleteItemIds[0] != 101 {
		t.Fatalf("unexpected delete payload: %+v", v2.InventoryPatch.DeleteItemIds)
	}

	r2 := newSyncTestRole(1004)
	r2.PbRole.InventoryInfo.ItemMap[202] = &g1_protocol.PbItem{Id: 202, Count: 8}
	r2.MarkInventoryDirty(202, true)
	r2.MarkInventoryDirty(202, false)

	_, _, actualPatchMask, _, v2 = r2.prepareSyncPayload(true)
	if actualPatchMask != g1_protocol.ERoleSectionFlag_INVENTORY_INFO {
		t.Fatalf("expected inventory patch mask, got %v", actualPatchMask)
	}
	if len(v2.InventoryPatch.DeleteItemIds) != 0 {
		t.Fatalf("delete should be dropped after upsert, got %+v", v2.InventoryPatch.DeleteItemIds)
	}
	if len(v2.InventoryPatch.UpsertItems) != 1 || v2.InventoryPatch.UpsertItems[0].GetId() != 202 {
		t.Fatalf("unexpected upsert payload: %+v", v2.InventoryPatch.UpsertItems)
	}
}

func TestShouldFlushPersistNowHonorsDebounce(t *testing.T) {
	// 通过 conf 加载配置设置 debounce（被测代码从 conf 读取，不再依赖 gconf 全局）。
	conf.ResetForTest()
	if err := conf.LoadBytes([]byte("mainsvr:\n  capacity:\n    role_persist_debounce_sec: 10\n"), ".yaml"); err != nil {
		t.Fatalf("conf.LoadBytes: %v", err)
	}

	r := newSyncTestRole(1005)
	r.needPersist = true
	r.persistDirtySince = 100

	if r.shouldFlushPersistNow(109, false) {
		t.Fatalf("persist flush should be debounced before threshold")
	}
	if !r.shouldFlushPersistNow(110, false) {
		t.Fatalf("persist flush should trigger at debounce threshold")
	}
	if !r.shouldFlushPersistNow(101, true) {
		t.Fatalf("force flush should ignore debounce threshold")
	}
}
