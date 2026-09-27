package role

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func (r *Role) GuideCompleted(id int32) int {
	// 是否已经存在
	for _, v := range r.PbRole.GuideInfo.IdLis {
		if v == id {
			return int(g1_protocol.ErrorCode_ERR_GUIDE_IS_EXIST)
		}
	}
	r.PbRole.GuideInfo.IdLis = append(r.PbRole.GuideInfo.IdLis, id)
	r.TouchGuideInfo("guide")
	return 0
}

func (r *Role) GuideInProgress(id int32) int {
	r.PbRole.GuideInfo.CurId = id
	r.TouchGuideInfo("guide")
	return 0
}

// ============================================================
// 引导段组件
// ============================================================

type GuideComponent struct{ sectionBase }

func NewGuideComponent() *GuideComponent { return &GuideComponent{} }

func (c *GuideComponent) Name() string { return "guide" }

func (c *GuideComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_GUIDE_INFO
}

func (c *GuideComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *GuideComponent) OnDestroy()            {}

// InitField 保持原 RoleInitField 语义：引导段不做 nil 兜底（按需懒建）。
func (c *GuideComponent) InitField(uid uint64) {}

func (c *GuideComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_GUIDE_INFO, reason)
}

func guideSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_GUIDE_INFO, "guide",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleGuideInfo { return i.GuideInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleGuideInfo) { i.GuideInfo = m },
		func() *g1_protocol.RoleGuideInfo { return new(g1_protocol.RoleGuideInfo) })
}
