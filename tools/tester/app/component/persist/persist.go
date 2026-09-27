// Package persist 角色持久化（DAL 三层）回归测试组件。
//
// 覆盖目标：登录全量同步 → 变更增量同步（patch/full mask）→ 重登/登出后
// 数据保持 → L3 权威恢复（phase=write/verify，配合 tools/cmd/dalprobe 清 L2）。
// 本地缓存以"登录全量同步"为基准建立，每步操作后按 Rule 6 先建预期再比对。
//
// 测试数据来源（真实配置表，与 inventory 组件同源）：
//   - 20101001：普通可用道具（规则2，可使用无 Getuse）
//   - 30100001：金币兑换物（Sale=100，可售/可分解）
package persist

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/tools/tester/app/component"
	"github.com/Iori372552686/GoOne/tools/tester/internal/session"
	"github.com/Iori372552686/GoOne/tools/tester/internal/testcfg"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

func init() {
	component.Register("persist", func() component.TesterComponent {
		return &PersistComponent{}
	})
}

// 真实配置表道具 ID（与 inventory 组件用例一致，勿凭空编造）。
const (
	itemUsable   = int32(20101001) // 普通道具：可添加、可显式使用
	itemSellable = int32(30100001) // 金币兑换物：Sale=100
)

// ModuleCfg [modules.persist] 私有参数。
type ModuleCfg struct {
	// Phase：solo（默认，单进程全链路用例）/ write / verify（L3 权威两阶段，
	// 中间由 dalprobe 清除 L2，验证从 L3 恢复）。
	Phase string `toml:"phase" json:"phase"`
	// MarkerCount write 阶段写入的标记道具数量（verify 阶段必须原样读回）。
	MarkerCount int64 `toml:"marker_count" json:"marker_count"`
}

// PersistComponent 持久化回归组件。
type PersistComponent struct {
	actorID   int
	accountID string
	userID    int64
	sender    component.MessageSender
	requester component.Requester
	cfg       *testcfg.Config

	module ModuleCfg

	mu               sync.Mutex
	syncVersion      uint64 // 每收到一次同步推送 +1（条件等待用）
	gold             int64  // 本地缓存：金币（BasicInfo.Gold）
	name             string // 本地缓存：角色名
	items            map[int32]int64
	lastFull         int32 // 最近一次 V2 同步的 full_section_mask
	lastPatch        int32 // 最近一次 V2 同步的 patch_section_mask
	lastBasicTouched bool  // 最近一次同步是否触碰 BASIC 段（v1 无 mask，按字段存在性判定）
	obtainSeen       bool  // 收到过 CMD_SC_OBTAIN_NOTICE
}

func (c *PersistComponent) Name() string { return "persist" }

func (c *PersistComponent) OnInit(ctx *component.ComponentContext) error {
	c.actorID = ctx.ActorID
	c.accountID = ctx.AccountID
	c.userID = ctx.UserID
	c.sender = ctx.Sender
	c.requester = ctx.Requester
	c.cfg = ctx.Cfg
	c.items = make(map[int32]int64)

	_ = ctx.Cfg.DecodeModule(c.Name(), &c.module)
	if c.module.Phase == "" {
		c.module.Phase = "solo"
	}
	if c.module.MarkerCount == 0 {
		// 20101001（规则2 普通道具）MaxOwnCount=9999：标记数须在限内，
		// 又要显著高于 solo 阶段的累计噪声（~几十）。
		c.module.MarkerCount = 5000
	}
	log.Printf("[Actor %d][Persist] init phase=%s marker=%d", c.actorID, c.module.Phase, c.module.MarkerCount)
	return nil
}

func (c *PersistComponent) OnConnected() error            { return nil }
func (c *PersistComponent) OnAccountLogin(a string) error { c.accountID = a; return nil }
func (c *PersistComponent) OnRoleLogin(uid int64) error   { c.userID = uid; return nil }

// OnMessage 捕获服务端主动同步推送，维护本地缓存。
// 返回 true 的语义仅为"已消费日志"，框架不据此拦截其他组件。
func (c *PersistComponent) OnMessage(cmd uint32, data []byte) bool {
	switch g1_protocol.CMD(cmd) {
	case g1_protocol.CMD_SC_SYNC_USER_DATA:
		msg := &g1_protocol.ScSyncUserData{}
		if err := proto.Unmarshal(data, msg); err != nil {
			log.Printf("[Actor %d][Persist] sync v1 unmarshal: %v", c.actorID, err)
			return false
		}
		c.applyFull(msg.GetRoleInfo(), int32(g1_protocol.ERoleSectionFlag_ALL), 0)
	case g1_protocol.CMD_SC_SYNC_USER_DATA_V2:
		msg := &g1_protocol.ScSyncUserDataV2{}
		if err := proto.Unmarshal(data, msg); err != nil {
			log.Printf("[Actor %d][Persist] sync v2 unmarshal: %v", c.actorID, err)
			return false
		}
		c.applyFull(msg.GetRoleInfo(), msg.GetFullSectionMask(), msg.GetPatchSectionMask())
		c.applyPatch(msg)
	case g1_protocol.CMD_SC_OBTAIN_NOTICE:
		c.mu.Lock()
		c.obtainSeen = true
		c.mu.Unlock()
	default:
		return false
	}
	return true
}

// RunTests 按阶段分发。
func (c *PersistComponent) RunTests(ctx context.Context) error {
	switch c.module.Phase {
	case "write":
		return c.runPhaseWrite(ctx)
	case "verify":
		return c.runPhaseVerify(ctx)
	default:
		return c.runSolo(ctx)
	}
}

// RunStress 压测正常路径：加道具 + 查询 + 心跳（轻量组合）。
func (c *PersistComponent) RunStress(ctx context.Context) error {
	if _, err := c.gmAddItem(ctx, itemUsable, 1); err != nil {
		return err
	}
	_, _ = c.queryBackpack(ctx, 0, 1, 10)
	return c.heartbeat(ctx)
}

// ---------------- 本地缓存维护 ----------------

func (c *PersistComponent) applyFull(info *g1_protocol.RoleInfo, fullMask, patchMask int32) {
	if info == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncVersion++
	c.lastFull, c.lastPatch = fullMask, patchMask
	// v1（legacy）同步无 mask 字段：以 section 字段是否下发判定；
	// v2 有 fullMask>0 时按位判定（fullMask==ALL 的 v1 路径会带全字段，同样成立）。
	c.lastBasicTouched = info.GetBasicInfo() != nil ||
		(fullMask != 0 && fullMask != int32(g1_protocol.ERoleSectionFlag_ALL) &&
			fullMask&int32(g1_protocol.ERoleSectionFlag_BASIC_INFO) != 0)
	if b := info.GetBasicInfo(); b != nil {
		c.gold = b.GetGold()
		if b.GetName() != "" {
			c.name = b.GetName()
		}
	}
	if inv := info.GetInventoryInfo(); inv != nil {
		// 全量同步含 INVENTORY 段时按服务器重建该段缓存：缺席即已删除
		//（出售/使用归零后服务器从 ItemMap 移除；V1 无 mask、V2 有 full 位，
		// 二者 InventoryInfo 非 nil 都代表该段被下发）。纯增量合并会漏删。
		for id := range c.items {
			delete(c.items, id)
		}
		for id, it := range inv.GetItemMap() {
			if it.GetCount() > 0 {
				c.items[id] = it.GetCount()
			} else {
				delete(c.items, id)
			}
		}
	}
}

func (c *PersistComponent) applyPatch(msg *g1_protocol.ScSyncUserDataV2) {
	if p := msg.GetInventoryPatch(); p != nil {
		c.mu.Lock()
		c.syncVersion++
		for _, it := range p.GetUpsertItems() {
			if it.GetCount() > 0 {
				c.items[it.GetId()] = it.GetCount()
			} else {
				delete(c.items, it.GetId())
			}
		}
		for _, id := range p.GetDeleteItemIds() {
			delete(c.items, id)
		}
		c.mu.Unlock()
	}
}

// snapshot 返回条件等待用的副本断言函数集合。
func (c *PersistComponent) goldNow() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.gold
}

func (c *PersistComponent) itemCount(id int32) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items[id]
}

func (c *PersistComponent) nameNow() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.name
}

// waitSyncCond 轮询等待同步状态满足条件（推送是异步的）。
func (c *PersistComponent) waitSyncCond(ctx context.Context, desc string, cond func() bool) error {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("等待同步超时: %s", desc)
}

// ---------------- 协议封装 ----------------

func (c *PersistComponent) isErr(code g1_protocol.ErrorCode) bool {
	return session.IsErrCode(int32(code))
}

func (c *PersistComponent) login(ctx context.Context) (*g1_protocol.LoginRsp, error) {
	req := &g1_protocol.LoginReq{
		Account:   c.accountID,
		LoginType: "guest",
		ChannelId: 1,
		DeviceOs:  "tester",
	}
	if c.cfg.Player.Token != "" {
		req.Token = c.cfg.Player.Token
	}
	rsp := &g1_protocol.LoginRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGIN_REQ), req, rsp, 15*time.Second); err != nil {
		return nil, err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return nil, fmt.Errorf("login failed: code=%d", rsp.GetRet().GetCode())
	}
	// 登录响应的 RoleInfo 即首次全量同步，作为本地缓存基准。
	c.applyFull(rsp.GetRoleInfo(), int32(g1_protocol.ERoleSectionFlag_ALL), 0)
	return rsp, nil
}

func (c *PersistComponent) logout(ctx context.Context) error {
	rsp := &g1_protocol.LogoutRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_LOGOUT_REQ),
		&g1_protocol.LogoutReq{Reason: "tester"}, rsp, 10*time.Second); err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("logout failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

func (c *PersistComponent) heartbeat(ctx context.Context) error {
	rsp := &g1_protocol.HeartBeatRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ),
		&g1_protocol.HeartBeatReq{ClientNowMs: time.Now().UnixMilli()}, rsp, 10*time.Second); err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("heartbeat failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

func (c *PersistComponent) gmAddItem(ctx context.Context, id int32, count int64) (*g1_protocol.GMAddItemRsp, error) {
	rsp := &g1_protocol.GMAddItemRsp{Ret: &g1_protocol.Ret{}}
	err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GM_ADD_ITEM_REQ),
		&g1_protocol.GMAddItemReq{Id: id, Count: count}, rsp, 10*time.Second)
	return rsp, err
}

func (c *PersistComponent) queryBackpack(ctx context.Context, bagType, pageIdx, pageSize int32) (int32, error) {
	rsp := &g1_protocol.QueryBackpackRsp{}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_BACKPACK_QUERY_REQ),
		&g1_protocol.QueryBackpackReq{BagType: bagType, PageIdx: pageIdx, PageSize: pageSize}, rsp, 10*time.Second); err != nil {
		return 0, err
	}
	return rsp.GetTotal(), nil
}

func (c *PersistComponent) gmGetRole(ctx context.Context) (*g1_protocol.RoleInfo, error) {
	rsp := &g1_protocol.GMGetRoleRsp{}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_GM_GET_ROLE_REQ),
		&g1_protocol.GMGetRoleReq{}, rsp, 10*time.Second); err != nil {
		return nil, err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return nil, fmt.Errorf("gm get role failed: code=%d", rsp.GetRet().GetCode())
	}
	return rsp.GetRoleInfo(), nil
}

func (c *PersistComponent) changeName(ctx context.Context, name string) error {
	rsp := &g1_protocol.ChangeNameRsp{Ret: &g1_protocol.Ret{}}
	if err := c.requester.RequestProto(ctx, uint32(g1_protocol.CMD_MAIN_CHANGE_NAME_REQ),
		&g1_protocol.ChangeNameReq{Name: name}, rsp, 10*time.Second); err != nil {
		return err
	}
	if c.isErr(rsp.GetRet().GetCode()) {
		return fmt.Errorf("change name failed: code=%d", rsp.GetRet().GetCode())
	}
	return nil
}

// runTests 通用执行器（沿用 login/inventory 风格）。
func (c *PersistComponent) runTests(ctx context.Context, tests []struct {
	name string
	fn   func(ctx context.Context) error
}) error {
	log.Printf("[Actor %d][Persist] ===== phase=%s tests start =====", c.actorID, c.module.Phase)
	for _, t := range tests {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		log.Printf("[Actor %d][Persist] --- %s ---", c.actorID, t.name)
		if err := t.fn(ctx); err != nil {
			return fmt.Errorf("%s: %w", t.name, err)
		}
		log.Printf("[Actor %d][Persist] --- %s PASSED ---", c.actorID, t.name)
	}
	log.Printf("[Actor %d][Persist] ===== phase=%s ALL PASSED =====", c.actorID, c.module.Phase)
	return nil
}

var _ component.TesterComponent = (*PersistComponent)(nil)
