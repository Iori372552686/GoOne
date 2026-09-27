package persist

import (
	"context"
	"fmt"
	"log"
	"time"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// runPhaseWrite L3 权威验证·写阶段：登录 → 写入标记数据 → 登出（强制 L2+L3 flush）。
// 退出后由编排脚本（dalprobe wipe-l2）清除 L2，再跑 verify 阶段。
func (c *PersistComponent) runPhaseWrite(ctx context.Context) error {
	if _, err := c.login(ctx); err != nil {
		return fmt.Errorf("write 阶段登录: %w", err)
	}

	// 标记 1：道具数量（inventory section）。检查返回码——超 MaxOwnCount 等
	// 业务拒绝会立即失败，避免误判为同步丢失。
	rsp, err := c.gmAddItem(ctx, itemUsable, c.module.MarkerCount)
	if err != nil {
		return fmt.Errorf("写入标记道具: %w", err)
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("写入标记道具被拒绝: code=%d（标记数 %d 是否超出道具 MaxOwnCount？）",
			rsp.GetRet().GetCode(), c.module.MarkerCount)
	}
	if err := c.waitSyncCond(ctx, "标记道具同步", func() bool {
		return c.itemCount(itemUsable) >= c.module.MarkerCount
	}); err != nil {
		return fmt.Errorf("标记道具未同步: got=%d want>=%d", c.itemCount(itemUsable), c.module.MarkerCount)
	}

	// 标记 2：角色名（basic section，含改名唯一性）。
	markerName := fmt.Sprintf("l3w_%d_%d", c.userID, time.Now().UnixNano()%1000000)
	if err := c.changeName(ctx, markerName); err != nil {
		return fmt.Errorf("写入标记名: %w", err)
	}

	gold := c.goldNow()
	item := c.itemCount(itemUsable)
	log.Printf("[Actor %d][Persist] WRITE markers: name=%q gold=%d item(%d)=%d",
		c.actorID, markerName, gold, itemUsable, item)

	// 登出：触发 Logout 用例的强制保存（L2 增量 + L3 快照 one-way）。
	// 给 L3 投递一点在途时间（one-way，由停机顺序/重试兜底）。
	if err := c.logout(ctx); err != nil {
		return fmt.Errorf("write 阶段登出: %w", err)
	}
	time.Sleep(2 * time.Second)
	log.Printf("[Actor %d][Persist] WRITE phase done（dalprobe wipe-l2 后执行 verify）", c.actorID)
	return nil
}

// runPhaseVerify L3 权威验证·读阶段：L2 已被 dalprobe 清除，登录必须从 L3 恢复
// 标记数据。任何缺失都意味着 L3 快照不完整（TTL 安全性失效）。
func (c *PersistComponent) runPhaseVerify(ctx context.Context) error {
	rsp, err := c.login(ctx)
	if err != nil {
		// 关键判定：L2 空 + L3 有数据时登录失败 = 回源链路故障。
		return fmt.Errorf("verify 阶段登录失败（L3 回源链路故障？）: %w", err)
	}
	info := rsp.GetRoleInfo()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"V1_NameRecoveredFromL3", func() error {
			got := info.GetBasicInfo().GetName()
			// 名字是时间戳后缀标记，只校验前缀（账号可多次跑 write）。
			if len(got) < 4 || got[:4] != "l3w_" {
				return fmt.Errorf("角色名未从 L3 恢复: got=%q want prefix l3w_（BUG：basic 段快照丢失或回源失败）", got)
			}
			return nil
		}},
		{"V2_GoldRecoveredFromL3", func() error {
			got := info.GetCurrencyInfo().GetCurrencyMap()[int32(g1_protocol.EItemID_GOLD)]
			if got <= 0 {
				return fmt.Errorf("金币未从 L3 恢复: got=%d（BUG：重登变成新角色或快照过旧）", got)
			}
			return nil
		}},
		{"V3_InventoryRecoveredFromL3", func() error {
			got := info.GetInventoryInfo().GetItemMap()[itemUsable].GetCount()
			if got < c.module.MarkerCount {
				return fmt.Errorf("标记道具未从 L3 恢复: got=%d want>=%d（BUG：inventory 段快照丢失）",
					got, c.module.MarkerCount)
			}
			return nil
		}},
		{"V4_SectionsComplete", func() error {
			if info.GetRegisterInfo() == nil || info.GetLoginInfo() == nil || info.GetBasicInfo() == nil {
				return fmt.Errorf("L3 恢复后 section 不完整（回填缺段）")
			}
			return nil
		}},
	}
	log.Printf("[Actor %d][Persist] VERIFY: name=%q gold=%d item(%d)=%d",
		c.actorID, info.GetBasicInfo().GetName(),
		info.GetCurrencyInfo().GetCurrencyMap()[int32(g1_protocol.EItemID_GOLD)],
		itemUsable, info.GetInventoryInfo().GetItemMap()[itemUsable].GetCount())

	for _, t := range tests {
		log.Printf("[Actor %d][Persist] --- %s ---", c.actorID, t.name)
		if err := t.fn(); err != nil {
			return fmt.Errorf("%s: %w", t.name, err)
		}
		log.Printf("[Actor %d][Persist] --- %s PASSED ---", c.actorID, t.name)
	}
	log.Printf("[Actor %d][Persist] ===== L3 权威恢复验证通过 =====", c.actorID)
	return nil
}
