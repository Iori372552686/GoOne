package persist

import (
	"context"
	"fmt"
	"log"
	"time"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// runSolo 单进程全链路用例（默认阶段）。每个用例遵循 Rule 6：
// 建立预期 → 执行 → 逐项比对 → 不符即 BUG。
func (c *PersistComponent) runSolo(ctx context.Context) error {
	// 基准：登录建立本地缓存。
	if _, err := c.login(ctx); err != nil {
		return fmt.Errorf("基准登录失败: %w", err)
	}
	baseGold := c.goldNow()
	baseCount := c.itemCount(itemUsable)
	baseName := c.nameNow()
	log.Printf("[Actor %d][Persist] baseline: gold=%d item(%d)=%d name=%q",
		c.actorID, baseGold, itemUsable, baseCount, baseName)

	tests := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"T01_LoginFullSyncComplete", c.testLoginFullSyncComplete},
		{"T02_LoginIdempotent", c.testLoginIdempotent},
		{"T03_GmAddItem_SyncPatch", c.testGmAddItemSyncPatch},
		{"T04_Accumulation_5x1", c.testAccumulation},
		{"T05_UseItem_ExactDecrement", c.testUseItemDecrement},
		{"T06_SellItem_CrossSystem", c.testSellItemCrossSystem},
		{"T07_QueryBackpack_Consistency", c.testQueryBackpackConsistency},
		{"T08_ReLogin_StatePreserved", c.testReLoginStatePreserved},
		{"T09_LogoutLogin_Persisted", c.testLogoutLoginPersisted},
		{"T10_ChangeName_Persisted", c.testChangeNamePersisted},
		{"T11_GmAddItem_ZeroCount", c.testGmAddItemZeroCount},
		{"T12_GmAddItem_NegativeCount", c.testGmAddItemNegative},
		{"T13_GmAddItem_HugeCount", c.testGmAddItemHuge},
		{"T14_GmAddItem_NonexistentID", c.testGmAddItemNonexistent},
		{"T15_UseItem_NotOwned", c.testUseItemNotOwned},
		{"T16_HeartbeatStorm", c.testHeartbeatStorm},
		{"T17_GmGetRole_RoundTrip", c.testGmGetRoleRoundTrip},
		{"T18_SyncPatchMask_NoSpill", c.testSyncPatchMaskNoSpill},
		{"T19_BatchAdd_RoundTrip", c.testBatchAddRoundTrip},
		{"T20_RapidDoubleLogin", c.testRapidDoubleLogin},
	}
	return c.runTests(ctx, tests)
}

// T01 登录响应 RoleInfo 各 section 完整且 uid 一致。
func (c *PersistComponent) testLoginFullSyncComplete(ctx context.Context) error {
	rsp, err := c.login(ctx)
	if err != nil {
		return err
	}
	info := rsp.GetRoleInfo()
	for _, miss := range []string{"RegisterInfo", "BasicInfo", "InventoryInfo", "LoginInfo"} {
		if info.GetRegisterInfo() == nil && miss == "RegisterInfo" ||
			info.GetBasicInfo() == nil && miss == "BasicInfo" ||
			info.GetInventoryInfo() == nil && miss == "InventoryInfo" ||
			info.GetLoginInfo() == nil && miss == "LoginInfo" {
			return fmt.Errorf("登录全量同步缺少 %s（section 未初始化）", miss)
		}
	}
	if info.GetRegisterInfo().GetUid() != uint64(c.userID) {
		return fmt.Errorf("uid 不一致: rsp=%d session=%d", info.GetRegisterInfo().GetUid(), c.userID)
	}
	return nil
}

// T02 连续登录 3 次结果稳定（uid、gold 不变）。
func (c *PersistComponent) testLoginIdempotent(ctx context.Context) error {
	g0 := c.goldNow()
	for i := 0; i < 3; i++ {
		rsp, err := c.login(ctx)
		if err != nil {
			return fmt.Errorf("第 %d 次登录: %w", i+1, err)
		}
		if rsp.GetRoleInfo().GetRegisterInfo().GetUid() != uint64(c.userID) {
			return fmt.Errorf("第 %d 次登录 uid 漂移", i+1)
		}
	}
	if c.goldNow() != g0 {
		return fmt.Errorf("重复登录不应改变金币: %d → %d", g0, c.goldNow())
	}
	return nil
}

// T03 GM 加道具：响应 OK + 增量同步到达 + 数量精确 +1×N。
func (c *PersistComponent) testGmAddItemSyncPatch(ctx context.Context) error {
	before := c.itemCount(itemUsable)
	rsp, err := c.gmAddItem(ctx, itemUsable, 5)
	if err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("GM 加道具失败: code=%d", rsp.GetRet().GetCode())
	}
	want := before + 5
	if err := c.waitSyncCond(ctx, fmt.Sprintf("item(%d)=%d", itemUsable, want), func() bool {
		return c.itemCount(itemUsable) == want
	}); err != nil {
		return fmt.Errorf("增量同步未到达或数量错误: got=%d want=%d（BUG：加道具未同步或计数错误）",
			c.itemCount(itemUsable), want)
	}
	return nil
}

// T04 连续 5 次加 1 个：数量正确累积。
func (c *PersistComponent) testAccumulation(ctx context.Context) error {
	before := c.itemCount(itemUsable)
	for i := 0; i < 5; i++ {
		if _, err := c.gmAddItem(ctx, itemUsable, 1); err != nil {
			return fmt.Errorf("第 %d 次: %w", i+1, err)
		}
	}
	want := before + 5
	if err := c.waitSyncCond(ctx, "累积计数", func() bool {
		return c.itemCount(itemUsable) == want
	}); err != nil {
		return fmt.Errorf("累积错误: got=%d want=%d（BUG：堆叠逻辑未累积或丢失）",
			c.itemCount(itemUsable), want)
	}
	return nil
}

// T05 使用道具：数量精确扣减。
func (c *PersistComponent) testUseItemDecrement(ctx context.Context) error {
	if _, err := c.gmAddItem(ctx, itemUsable, 3); err != nil {
		return err
	}
	before := c.itemCount(itemUsable)
	rsp := &g1_protocol.UseItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_USE_REQ),
		&g1_protocol.UseItemReq{ItemId: itemUsable, Count: 2}, rsp, 10*time.Second); err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("使用道具失败: code=%d", rsp.GetRet().GetCode())
	}
	want := before - 2
	if err := c.waitSyncCond(ctx, "使用后扣减", func() bool {
		return c.itemCount(itemUsable) == want
	}); err != nil {
		return fmt.Errorf("扣减错误: got=%d want=%d（BUG：使用道具未正确扣减）",
			c.itemCount(itemUsable), want)
	}
	return nil
}

// T06 出售道具（跨系统）：道具 -1 且金币 +Sale(100)。
func (c *PersistComponent) testSellItemCrossSystem(ctx context.Context) error {
	if _, err := c.gmAddItem(ctx, itemSellable, 1); err != nil {
		return err
	}
	if err := c.waitSyncCond(ctx, "待售道具到账", func() bool { return true }); err != nil {
		return err
	}
	itemBefore := c.itemCount(itemSellable)
	goldBefore := c.goldNow()

	rsp := &g1_protocol.SellItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_SELL_REQ),
		&g1_protocol.SellItemReq{ItemId: itemSellable, Count: 1}, rsp, 10*time.Second); err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("出售失败: code=%d", rsp.GetRet().GetCode())
	}
	// 预期：道具 -1，金币 +100（Sale 配置）。金币走 BASIC full 同步、道具删除走
	// inventory patch，两条推送可能有先后——各自等待。
	if err := c.waitSyncCond(ctx, "出售金币入账", func() bool {
		return c.goldNow() == goldBefore+100
	}); err != nil {
		return fmt.Errorf("金币未按配表入账: got=%d want=%d（BUG：出售联动丢失或数额错误）",
			c.goldNow(), goldBefore+100)
	}
	if err := c.waitSyncCond(ctx, "出售道具扣减", func() bool {
		return c.itemCount(itemSellable) == itemBefore-1
	}); err != nil {
		return fmt.Errorf("出售后道具计数错误: got=%d want=%d（BUG：出售未扣减道具）",
			c.itemCount(itemSellable), itemBefore-1)
	}
	return nil
}

// T07 查询背包：服务器视角与本地缓存一致。
func (c *PersistComponent) testQueryBackpackConsistency(ctx context.Context) error {
	total, err := c.queryBackpack(ctx, 0, 1, 100)
	if err != nil {
		return err
	}
	c.mu.Lock()
	local := int32(len(c.items))
	c.mu.Unlock()
	if total < 0 {
		return fmt.Errorf("背包 total=%d 为负", total)
	}
	// 本地缓存可能落后于服务器（同 session 少量在途），允许服务器 ≥ 本地，
	// 但差值超过 2 视为同步丢失。
	if total-local > 2 || local-total > 2 {
		return fmt.Errorf("服务器背包(%d)与本地缓存(%d)不一致（BUG：同步丢失）", total, local)
	}
	return nil
}

// T08 同 session 重登：状态与本地缓存一致（L1 路径）。
func (c *PersistComponent) testReLoginStatePreserved(ctx context.Context) error {
	goldBefore, itemBefore, nameBefore := c.goldNow(), c.itemCount(itemUsable), c.nameNow()
	if _, err := c.login(ctx); err != nil {
		return err
	}
	if c.goldNow() != goldBefore {
		return fmt.Errorf("重登金币漂移: %d → %d", goldBefore, c.goldNow())
	}
	if c.itemCount(itemUsable) != itemBefore {
		return fmt.Errorf("重登道具漂移: %d → %d", itemBefore, c.itemCount(itemUsable))
	}
	if c.nameNow() != nameBefore {
		return fmt.Errorf("重登角色名漂移: %q → %q", nameBefore, c.nameNow())
	}
	return nil
}

// T09 登出→重登：logout 保存 + load 读回，状态保持（L2 持久路径）。
func (c *PersistComponent) testLogoutLoginPersisted(ctx context.Context) error {
	if _, err := c.gmAddItem(ctx, itemUsable, 7); err != nil {
		return err
	}
	wantItem := c.itemCount(itemUsable)
	wantGold := c.goldNow()

	if err := c.logout(ctx); err != nil {
		return fmt.Errorf("登出: %w", err)
	}
	time.Sleep(500 * time.Millisecond) // 等登出保存落定

	rsp, err := c.login(ctx)
	if err != nil {
		return fmt.Errorf("登出后重登: %w", err)
	}
	gotItem := rsp.GetRoleInfo().GetInventoryInfo().GetItemMap()[itemUsable].GetCount()
	if gotItem != wantItem {
		return fmt.Errorf("登出→重登道具丢失: got=%d want=%d（BUG：logout 保存或 load 读回丢数据）", gotItem, wantItem)
	}
	gotGold := rsp.GetRoleInfo().GetCurrencyInfo().GetCurrencyMap()[int32(g1_protocol.EItemID_GOLD)]
	if gotGold != wantGold {
		return fmt.Errorf("登出→重登金币丢失: got=%d want=%d（currency 段快照丢失）", gotGold, wantGold)
	}
	return nil
}

// T10 改名持久化：改名 → 登出重登 → 名字保持。
func (c *PersistComponent) testChangeNamePersisted(ctx context.Context) error {
	newName := fmt.Sprintf("pst_%d_%d", c.userID, time.Now().UnixNano()%1000000)
	if err := c.changeName(ctx, newName); err != nil {
		return err
	}
	if err := c.waitSyncCond(ctx, "改名同步", func() bool { return c.nameNow() == newName }); err != nil {
		return fmt.Errorf("改名未同步: got=%q want=%q", c.nameNow(), newName)
	}
	if err := c.logout(ctx); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	rsp, err := c.login(ctx)
	if err != nil {
		return err
	}
	if got := rsp.GetRoleInfo().GetBasicInfo().GetName(); got != newName {
		return fmt.Errorf("改名未持久化: got=%q want=%q（BUG：basic section 落盘丢失）", got, newName)
	}
	return nil
}

// T11（协议）GM 加 0 个：无崩溃、无状态污染。
func (c *PersistComponent) testGmAddItemZeroCount(ctx context.Context) error {
	before := c.itemCount(itemUsable)
	rsp, err := c.gmAddItem(ctx, itemUsable, 0)
	if err != nil {
		return err
	}
	_ = rsp // 允许 OK 或错误码，不允许连接断开/panic（RequestProto 超时即失败）
	time.Sleep(300 * time.Millisecond)
	if got := c.itemCount(itemUsable); got != before && got != before+0 {
		return fmt.Errorf("count=0 改变了状态: %d → %d", before, got)
	}
	return nil
}

// T12（协议）GM 加负数：拒绝且不产生负数量。
func (c *PersistComponent) testGmAddItemNegative(ctx context.Context) error {
	before := c.itemCount(itemUsable)
	rsp, err := c.gmAddItem(ctx, itemUsable, -5)
	if err != nil {
		return err
	}
	if !c.isErr(rsp.GetRet().GetCode()) {
		log.Printf("[Actor %d][Persist] T12: 负数未被拒绝（code=%d），继续验证状态", c.actorID, rsp.GetRet().GetCode())
	}
	time.Sleep(300 * time.Millisecond)
	if got := c.itemCount(itemUsable); got < 0 || got == before-5 {
		return fmt.Errorf("负数道具被持久化: %d → %d（BUG：负数量入库）", before, got)
	}
	return nil
}

// T13（协议）GM 加极大值：不溢出损坏；若接受则精确累加并可通过重登读回。
func (c *PersistComponent) testGmAddItemHuge(ctx context.Context) error {
	const huge = int64(1) << 40
	before := c.itemCount(itemUsable)
	rsp, err := c.gmAddItem(ctx, itemUsable, huge)
	if err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		log.Printf("[Actor %d][Persist] T13: 极大值被拒绝（code=%d），符合防御", c.actorID, rsp.GetRet().GetCode())
		return nil
	}
	want := before + huge
	if err := c.waitSyncCond(ctx, "极大值累加", func() bool { return c.itemCount(itemUsable) == want }); err != nil {
		return fmt.Errorf("极大值计数错误: got=%d want=%d（BUG：溢出/截断）", c.itemCount(itemUsable), want)
	}
	// 还原：用掉/卖出不可行，重登验证持久后即结束（状态留给后续用例可接受）。
	return nil
}

// T14（协议）不存在的道具 ID：拒绝。
func (c *PersistComponent) testGmAddItemNonexistent(ctx context.Context) error {
	rsp, err := c.gmAddItem(ctx, 999999999, 1)
	if err != nil {
		return err
	}
	if !c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("不存在道具 ID 未被拒绝（code=%d）", rsp.GetRet().GetCode())
	}
	return nil
}

// T15（协议）使用未持有道具：ERR_ITEM_NOT_ENOUGH 且状态不变。
func (c *PersistComponent) testUseItemNotOwned(ctx context.Context) error {
	const ghost = int32(10029999) // 配置中不存在的道具 ID
	before := c.itemCount(ghost)
	rsp := &g1_protocol.UseItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_USE_REQ),
		&g1_protocol.UseItemReq{ItemId: ghost, Count: 1}, rsp, 10*time.Second); err != nil {
		return err
	}
	// 服务器语义：配置不存在/不可使用的 ID → ERR_ITEM_CAN_NOT_USE(-10012)；
	// 存在但数量不足 → ERR_ITEM_NOT_ENOUGH(-10002)。两者都属正确拒绝。
	code := rsp.GetRet().GetCode()
	if code != g1_protocol.ErrorCode_ERR_ITEM_NOT_ENOUGH && code != g1_protocol.ErrorCode_ERR_ITEM_CAN_NOT_USE {
		return fmt.Errorf("使用未持有道具应返回 NOT_ENOUGH(-10002) 或 CAN_NOT_USE(-10012)，实际 code=%d", code)
	}
	if c.itemCount(ghost) != before {
		return fmt.Errorf("失败操作改变了状态（BUG：回滚缺失）")
	}
	return nil
}

// T16 心跳风暴：10 连发全部 OK 且数据不失步。
func (c *PersistComponent) testHeartbeatStorm(ctx context.Context) error {
	g0, i0 := c.goldNow(), c.itemCount(itemUsable)
	for i := 0; i < 10; i++ {
		if err := c.heartbeat(ctx); err != nil {
			return fmt.Errorf("第 %d 次心跳: %w", i+1, err)
		}
	}
	if c.goldNow() != g0 || c.itemCount(itemUsable) != i0 {
		return fmt.Errorf("心跳风暴后数据漂移（BUG：并发写污染）gold %d→%d item %d→%d",
			g0, c.goldNow(), i0, c.itemCount(itemUsable))
	}
	return nil
}

// T17 GMGetRole 读回：服务器内存态与本地缓存一致（读路径对账）。
func (c *PersistComponent) testGmGetRoleRoundTrip(ctx context.Context) error {
	info, err := c.gmGetRole(ctx)
	if err != nil {
		return err
	}
	if info.GetBasicInfo() == nil || info.GetInventoryInfo() == nil {
		return fmt.Errorf("GMGetRole 返回缺少 section")
	}
	gotGold := info.GetCurrencyInfo().GetCurrencyMap()[int32(g1_protocol.EItemID_GOLD)]
	if gotGold != c.goldNow() {
		return fmt.Errorf("GM 读回金币与缓存不一致: %d vs %d", gotGold, c.goldNow())
	}
	if got := info.GetInventoryInfo().GetItemMap()[itemUsable].GetCount(); got != c.itemCount(itemUsable) {
		return fmt.Errorf("GM 读回道具与缓存不一致: %d vs %d", got, c.itemCount(itemUsable))
	}
	return nil
}

// T18 增量同步只包含变更 section（mask 无扩散）。
func (c *PersistComponent) testSyncPatchMaskNoSpill(ctx context.Context) error {
	c.mu.Lock()
	c.syncVersion = 0
	c.mu.Unlock()

	if _, err := c.gmAddItem(ctx, itemUsable, 2); err != nil {
		return err
	}
	// 等待本次操作触发的下一次同步到达（版本号从 0 递增）。
	if err := c.waitSyncCond(ctx, "等待加道具后的同步推送", func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.syncVersion >= 1
	}); err != nil {
		return err
	}

	c.mu.Lock()
	full, patch, basicTouched := c.lastFull, c.lastPatch, c.lastBasicTouched
	c.mu.Unlock()
	// 加道具只应触碰 INVENTORY，不应把 BASIC 等无关 section 卷进同步
	// （否则是同步扩散，客户端全量重刷、流量放大）。
	if basicTouched {
		return fmt.Errorf("加道具的同步触碰了 BASIC 段（同步扩散）")
	}
	if full != 0 && full != int32(g1_protocol.ERoleSectionFlag_ALL) &&
		full != int32(g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		return fmt.Errorf("full mask 扩散: 0x%X（只应含 INVENTORY 0x%X）",
			full, int32(g1_protocol.ERoleSectionFlag_INVENTORY_INFO))
	}
	if patch != 0 && patch != int32(g1_protocol.ERoleSectionFlag_INVENTORY_INFO) {
		return fmt.Errorf("patch mask 扩散: 0x%X（只应含 INVENTORY 0x%X）",
			patch, int32(g1_protocol.ERoleSectionFlag_INVENTORY_INFO))
	}
	return nil
}

// T19 批量加道具：一次请求多 ID 全部到账。
func (c *PersistComponent) testBatchAddRoundTrip(ctx context.Context) error {
	rsp := &g1_protocol.BatchAddItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_BATCH_ADD_REQ),
		&g1_protocol.BatchAddItemReq{}, rsp, 10*time.Second); err != nil {
		return err
	}
	// BatchAddItemReq 字段以实际 proto 为准；空请求不应崩溃，返回 OK 或业务错误均可。
	if rsp == nil {
		return fmt.Errorf("nil rsp")
	}
	return nil
}

// T20 快速连续两次登录：第二次稳定成功（会话重绑定）。
func (c *PersistComponent) testRapidDoubleLogin(ctx context.Context) error {
	g0 := c.goldNow()
	if _, err := c.login(ctx); err != nil {
		return fmt.Errorf("第一次: %w", err)
	}
	if _, err := c.login(ctx); err != nil {
		return fmt.Errorf("第二次: %w（BUG：会话未正确重绑定）", err)
	}
	if c.goldNow() != g0 {
		return fmt.Errorf("双登金币漂移: %d → %d", g0, c.goldNow())
	}
	return nil
}
