package role

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

// 同步功能开放
func (r *Role) SyncOpenFuncData() {

}

func (r *Role) FuncIsOpen(openFuncId int32) bool {
	if r.PbRole.OpenFunInfo.IsAllOpen {
		return true
	}
	//return r.PbRole.OpenFunInfo.Data[openFuncId]
	for _, v := range r.PbRole.OpenFunInfo.Data {
		if v == openFuncId {
			return true
		}
	}
	return false
}

// ============================================================
// 功能开放段组件
// ============================================================

type OpenFuncComponent struct{ sectionBase }

func NewOpenFuncComponent() *OpenFuncComponent { return &OpenFuncComponent{} }

func (c *OpenFuncComponent) Name() string { return "open_func" }

func (c *OpenFuncComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO
}

func (c *OpenFuncComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *OpenFuncComponent) OnDestroy()            {}

func (c *OpenFuncComponent) InitField(uid uint64) {
	if c.role.PbRole.OpenFunInfo == nil {
		c.role.PbRole.OpenFunInfo = &g1_protocol.RoleOpenFunction{}
		c.role.PbRole.OpenFunInfo.IsAllOpen = true
	}
}

func (c *OpenFuncComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO, reason)
}

func openFuncSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_OPEN_FUNC_INFO, "open_func",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleOpenFunction { return i.OpenFunInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleOpenFunction) { i.OpenFunInfo = m },
		func() *g1_protocol.RoleOpenFunction { return new(g1_protocol.RoleOpenFunction) })
}
