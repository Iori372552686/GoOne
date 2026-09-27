package role

import (
	"github.com/Iori372552686/GoOne/module/gamedata/repository/mall"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func (r *Role) EnsureMallItem(confId int32) *g1_protocol.PbMallItem {
	info := r.PbRole.MallInfo

	if info.ItemMap == nil {
		info.ItemMap = make(map[int32]*g1_protocol.PbMallItem)
	}
	if info.ItemMap[confId] == nil {
		info.ItemMap[confId] = &g1_protocol.PbMallItem{ConfId: confId}
	}

	return info.ItemMap[confId]
}

func (r *Role) MallDailyRefresh() {
	info := r.PbRole.MallInfo
	for confId, v := range info.ItemMap {
		v.DailyBuyCount = 0
		r.MarkMallDirty(confId, false)
	}
}

func (r *Role) MallAddBuyCount(confId int32) {
	item := r.EnsureMallItem(confId)
	item.DailyBuyCount++
	item.TotalBuyCount++
	r.MarkMallDirty(confId, false)
}

func (r *Role) MallCheckBuyCondition(confId int32) g1_protocol.ErrorCode {
	conf := mall.GetMallById(confId)
	if conf == nil {
		return g1_protocol.ErrorCode_ERR_CONF
	}

	/*	now := int64(r.Now())
		if (conf.BeginTime > 0 && now < conf.BeginTime) ||
			(conf.EndTime > 0 && now > conf.EndTime) {
			return g1_protocol.ErrorCode_ERR_MALL_OUT_OF_TIME
		}*/

	item := r.EnsureMallItem(confId)
	if item.DailyBuyCount >= conf.DailyBuyLimit && conf.DailyBuyLimit > 0 {
		return g1_protocol.ErrorCode_ERR_MALL_DAILY_LIMIT
	}

	if item.TotalBuyCount >= conf.BuyLimit && conf.BuyLimit > 0 {
		return g1_protocol.ErrorCode_ERR_MALL_BUY_LIMIT
	}

	return 0
}

// ============================================================
// 商城段组件（增量档）
// ============================================================

type MallComponent struct {
	sectionBase

	mallUpserts int32Set
	mallDeletes int32Set
}

func NewMallComponent() *MallComponent { return &MallComponent{} }

func (c *MallComponent) Name() string { return "mall" }

func (c *MallComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_MALL_INFO
}

func (c *MallComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *MallComponent) OnDestroy()           {}

func (c *MallComponent) InitField(uid uint64) {
	if c.role.PbRole.MallInfo == nil {
		c.role.PbRole.MallInfo = &g1_protocol.RoleMallInfo{}
	}
}

func (c *MallComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_MALL_INFO, reason)
}

func (c *MallComponent) MarkDirty(confID int32, deleted bool) {
	c.role.markPatchSection(g1_protocol.ERoleSectionFlag_MALL_INFO)
	if deleted {
		delInt32Value(&c.mallUpserts, confID)
		setInt32Value(&c.mallDeletes, confID)
	} else {
		delInt32Value(&c.mallDeletes, confID)
		setInt32Value(&c.mallUpserts, confID)
	}
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_MALL_INFO, "mall")
}

func (c *MallComponent) BuildPatch(dst *g1_protocol.ScSyncUserDataV2) bool {
	patch := &g1_protocol.RoleMallPatch{}
	for _, confID := range sortedInt32Values(c.mallUpserts) {
		if c.role.PbRole.MallInfo == nil || c.role.PbRole.MallInfo.ItemMap == nil {
			continue
		}
		if item := c.role.PbRole.MallInfo.ItemMap[confID]; item != nil {
			patch.UpsertItems = append(patch.UpsertItems, item)
		}
	}
	patch.DeleteConfIds = sortedInt32Values(c.mallDeletes)
	if len(patch.UpsertItems) == 0 && len(patch.DeleteConfIds) == 0 {
		return false
	}
	dst.MallPatch = patch
	return true
}

func (c *MallComponent) ClearDirty() {
	c.mallUpserts = nil
	c.mallDeletes = nil
}

func mallSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_MALL_INFO, "mall",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleMallInfo { return i.MallInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleMallInfo) { i.MallInfo = m },
		func() *g1_protocol.RoleMallInfo { return new(g1_protocol.RoleMallInfo) })
}
