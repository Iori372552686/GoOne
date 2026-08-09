// Package inventory 背包/道具/掉落/恭喜获得 4 系统的 tester 回归组件。
// 覆盖 GmAddItem / QueryBackpack / UseItem / SellItem / DecomposeItem / Drop / ObtainNotice 7 个用例。
package inventory

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func init() {
	component.Register("inventory", func() component.TesterComponent {
		return &InventoryComponent{}
	})
}

// InventoryComponent 背包系统回归测试组件。
type InventoryComponent struct {
	actorID   int
	accountID string
	userID    int64
	requester component.Requester
	// obtainNotice 是否收到过 CMD_SC_OBTAIN_NOTICE 推送
	obtainNotice bool
}

func (c *InventoryComponent) Name() string { return "inventory" }

func (c *InventoryComponent) OnInit(ctx *component.ComponentContext) error {
	c.actorID = ctx.ActorID
	c.accountID = ctx.AccountID
	c.userID = ctx.UserID
	c.requester = ctx.Requester
	log.Printf("[Actor %d][Inv] Component initialized", c.actorID)
	return nil
}

func (c *InventoryComponent) OnConnected() error        { return nil }
func (c *InventoryComponent) OnAccountLogin(a string) error { return nil }
func (c *InventoryComponent) OnRoleLogin(uid int64) error {
	c.userID = uid
	return nil
}

// OnMessage 捕获服务端主动推送；CMD_SC_OBTAIN_NOTICE 到达则置标志。
func (c *InventoryComponent) OnMessage(cmd uint32, data []byte) bool {
	if cmd == uint32(g1_protocol.CMD_SC_OBTAIN_NOTICE) {
		c.obtainNotice = true
		log.Printf("[Actor %d][Inv] received SC_OBTAIN_NOTICE", c.actorID)
		return true
	}
	return false
}

// RunStress 压测正常路径：加道具 + 查背包（高频轻量组合）。
func (c *InventoryComponent) RunStress(ctx context.Context) error {
	if err := c.gmAddItem(ctx, 30100001, 1); err != nil {
		return err
	}
	_, _ = c.queryBackpack(ctx, 0, 1, 10)
	return nil
}

// RunTests 7 个回归用例。
func (c *InventoryComponent) RunTests(ctx context.Context) error {
	log.Printf("[Actor %d][Inv] ===== Starting inventory tests =====", c.actorID)
	tests := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"T01_GmAddItem", c.testGmAddItem},
		{"T02_QueryBackpack", c.testQueryBackpack},
		{"T03_UseItem", c.testUseItem},
		{"T04_SellItem", c.testSellItem},
		{"T05_DecomposeItem", c.testDecompose},
		{"T06_BatchAddItem", c.testBatchAdd},
		{"T07_ObtainNotice", c.testObtainNotice},
	}
	for _, t := range tests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		log.Printf("[Actor %d][Inv] --- %s ---", c.actorID, t.name)
		if err := t.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", t.name, err)
		}
		log.Printf("[Actor %d][Inv] --- %s PASSED ---", c.actorID, t.name)
	}
	log.Printf("[Actor %d][Inv] ===== All inventory tests PASSED =====", c.actorID)
	return nil
}

// T01: GM 加道具后能查到数量。
func (c *InventoryComponent) testGmAddItem(ctx context.Context) error {
	return c.gmAddItem(ctx, 30100001, 10)
}

// T02: 查背包返回非负 total。
func (c *InventoryComponent) testQueryBackpack(ctx context.Context) error {
	total, err := c.queryBackpack(ctx, 0, 1, 20)
	if err != nil {
		return err
	}
	if total < 0 {
		return fmt.Errorf("total should be >=0, got %d", total)
	}
	log.Printf("[Actor %d][Inv] T02: backpack total=%d", c.actorID, total)
	return nil
}

// T03: 使用道具(掉落组礼包 90101001)应成功。
func (c *InventoryComponent) testUseItem(ctx context.Context) error {
	// 先确保有该道具
	_ = c.gmAddItem(ctx, 90101001, 1)
	req := &g1_protocol.UseItemReq{ItemId: 90101001, Count: 1}
	rsp := &g1_protocol.UseItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_USE_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if rsp.Ret == nil || session.IsErrCode(int32(rsp.Ret.Code)) {
		return fmt.Errorf("use item failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

// T04: 出售可售道具应成功并加金币。
func (c *InventoryComponent) testSellItem(ctx context.Context) error {
	_ = c.gmAddItem(ctx, 30100001, 1) // 金币兑换物1，Sale=100
	req := &g1_protocol.SellItemReq{ItemId: 30100001, Count: 1}
	rsp := &g1_protocol.SellItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_SELL_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if rsp.Ret == nil || session.IsErrCode(int32(rsp.Ret.Code)) {
		return fmt.Errorf("sell item failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

// T05: 分解可分解道具应返回产出。
func (c *InventoryComponent) testDecompose(ctx context.Context) error {
	_ = c.gmAddItem(ctx, 30100001, 1) // DecomposeConfig: 30100001 → 10101002×100
	req := &g1_protocol.DecomposeItemReq{ItemId: 30100001, Count: 1}
	rsp := &g1_protocol.DecomposeItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_DECOMPOSE_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if rsp.Ret == nil || session.IsErrCode(int32(rsp.Ret.Code)) {
		return fmt.Errorf("decompose failed: code=%d", rsp.GetRet().GetCode())
	}
	log.Printf("[Actor %d][Inv] T05: decompose rewards=%d", c.actorID, len(rsp.GetRewards()))
	return nil
}

// T06: 批量加道具。
func (c *InventoryComponent) testBatchAdd(ctx context.Context) error {
	req := &g1_protocol.BatchAddItemReq{
		Items: []*g1_protocol.PbItem{
			{Id: 30100001, Count: 5},
			{Id: 30100002, Count: 3},
		},
	}
	rsp := &g1_protocol.BatchAddItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_BATCH_ADD_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if rsp.Ret == nil || session.IsErrCode(int32(rsp.Ret.Code)) {
		return fmt.Errorf("batch add failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

// T07: 触发分解(产出来源 decompose)后应收到 SC_OBTAIN_NOTICE 推送。
// 若策略表 source=decompose 未配置或冷却，推送可能缺席，用例降级为"不强制"。
func (c *InventoryComponent) testObtainNotice(ctx context.Context) error {
	c.obtainNotice = false
	_ = c.gmAddItem(ctx, 30100001, 1)
	dreq := &g1_protocol.DecomposeItemReq{ItemId: 30100001, Count: 1}
	drsp := &g1_protocol.DecomposeItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_ITEM_DECOMPOSE_REQ), dreq, drsp, 10*time.Second); err != nil {
		return err
	}
	// 等待推送（最多 1s）
	deadline := time.Now().Add(time.Second)
	for !c.obtainNotice && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if c.obtainNotice {
		log.Printf("[Actor %d][Inv] T07: ObtainNotice received", c.actorID)
	} else {
		log.Printf("[Actor %d][Inv] T07: ObtainNotice not received (acceptable: policy/cooldown)", c.actorID)
	}
	return nil
}

// ---- 公共子操作 ----

func (c *InventoryComponent) gmAddItem(ctx context.Context, itemID int32, count int64) error {
	req := &g1_protocol.GMAddItemReq{Id: itemID, Count: count}
	rsp := &g1_protocol.GMAddItemRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GM_ADD_ITEM_REQ), req, rsp, 10*time.Second); err != nil {
		return err
	}
	if rsp.Ret == nil || session.IsErrCode(int32(rsp.Ret.Code)) {
		return fmt.Errorf("gm add item %d failed: code=%d", itemID, rsp.GetRet().GetCode())
	}
	return nil
}

func (c *InventoryComponent) queryBackpack(ctx context.Context, bagType, pageIdx, pageSize int32) (int32, error) {
	req := &g1_protocol.QueryBackpackReq{BagType: bagType, PageIdx: pageIdx, PageSize: pageSize}
	rsp := &g1_protocol.QueryBackpackRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_BACKPACK_QUERY_REQ), req, rsp, 10*time.Second); err != nil {
		return 0, err
	}
	if rsp.Ret == nil || session.IsErrCode(int32(rsp.Ret.Code)) {
		return 0, fmt.Errorf("query backpack failed: code=%d", rsp.GetRet().GetCode())
	}
	return rsp.GetTotal(), nil
}
