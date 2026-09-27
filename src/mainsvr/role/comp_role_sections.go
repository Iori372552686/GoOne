package role

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/util/convert"
)

// 注册段组件。InitField 语义与原 RoleInitField 的对应分支保持一致。

type RegisterComponent struct{ sectionBase }

func NewRegisterComponent() *RegisterComponent { return &RegisterComponent{} }

func (c *RegisterComponent) Name() string { return "register" }

func (c *RegisterComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_REGISTER_INFO
}

func (c *RegisterComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *RegisterComponent) OnDestroy()            {}

func (c *RegisterComponent) InitField(uid uint64) {
	if c.role.PbRole.RegisterInfo != nil {
		return
	}
	c.role.PbRole.RegisterInfo = &g1_protocol.RoleRegisterInfo{}
	c.role.PbRole.RegisterInfo.Uid = uid
	c.role.PbRole.RegisterInfo.RegisterTime = datetime.Now()
}

func (c *RegisterComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_REGISTER_INFO, reason)
}

func registerSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_REGISTER_INFO, "register",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleRegisterInfo { return i.RegisterInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleRegisterInfo) { i.RegisterInfo = m },
		func() *g1_protocol.RoleRegisterInfo { return new(g1_protocol.RoleRegisterInfo) })
}

// 登录段组件。

type LoginComponent struct{ sectionBase }

func NewLoginComponent() *LoginComponent { return &LoginComponent{} }

func (c *LoginComponent) Name() string { return "login" }

func (c *LoginComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_LOGIN_INFO
}

func (c *LoginComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *LoginComponent) OnDestroy()            {}

func (c *LoginComponent) InitField(uid uint64) {
	if c.role.PbRole.LoginInfo == nil {
		c.role.PbRole.LoginInfo = &g1_protocol.RoleLoginInfo{}
	}
}

func (c *LoginComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_LOGIN_INFO, reason)
}

func loginSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_LOGIN_INFO, "login",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleLoginInfo { return i.LoginInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleLoginInfo) { i.LoginInfo = m },
		func() *g1_protocol.RoleLoginInfo { return new(g1_protocol.RoleLoginInfo) })
}

// 基础段组件：名称/等级/经验与货币标量（货币将在货币组件落地后停写）。

type BasicComponent struct{ sectionBase }

func NewBasicComponent() *BasicComponent { return &BasicComponent{} }

func (c *BasicComponent) Name() string { return "basic" }

func (c *BasicComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_BASIC_INFO
}

func (c *BasicComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *BasicComponent) OnDestroy()            {}

func (c *BasicComponent) InitField(uid uint64) {
	if c.role.PbRole.BasicInfo != nil {
		return
	}
	c.role.PbRole.BasicInfo = &g1_protocol.RoleBasicInfo{}
	c.role.PbRole.BasicInfo.Level = 1
	c.role.PbRole.BasicInfo.Name = "Player" + convert.Int64ToString(int64(uid))
	c.role.PbRole.BasicInfo.FreeCnt = 1
	// 货币/资源初值迁移至 CurrencyComponent.InitField（RoleCurrencyInfo）。
}

func (c *BasicComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_BASIC_INFO, reason)
}

func basicSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_BASIC_INFO, "basic",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleBasicInfo { return i.BasicInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleBasicInfo) { i.BasicInfo = m },
		func() *g1_protocol.RoleBasicInfo { return new(g1_protocol.RoleBasicInfo) })
}

// 主线任务段组件（当前无业务字段，仅保段占位）。

type MainTaskComponent struct{ sectionBase }

func NewMainTaskComponent() *MainTaskComponent { return &MainTaskComponent{} }

func (c *MainTaskComponent) Name() string { return "main_task" }

func (c *MainTaskComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO
}

func (c *MainTaskComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *MainTaskComponent) OnDestroy()            {}

func (c *MainTaskComponent) InitField(uid uint64) {
	if c.role.PbRole.MainTaskInfo == nil {
		c.role.PbRole.MainTaskInfo = &g1_protocol.RoleMainTaskInfo{}
	}
}

func (c *MainTaskComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO, reason)
}

func mainTaskSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_MAIN_TASK_INFO, "main_task",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleMainTaskInfo { return i.MainTaskInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleMainTaskInfo) { i.MainTaskInfo = m },
		func() *g1_protocol.RoleMainTaskInfo { return new(g1_protocol.RoleMainTaskInfo) })
}

// 公会段组件（预留空壳：原 RoleInitField 即不初始化该段，保持一致）。

type GuildComponent struct{ sectionBase }

func NewGuildComponent() *GuildComponent { return &GuildComponent{} }

func (c *GuildComponent) Name() string { return "guild" }

func (c *GuildComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_GUILD_INFO
}

func (c *GuildComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *GuildComponent) OnDestroy()            {}

func (c *GuildComponent) InitField(uid uint64) {}

func (c *GuildComponent) Touch(reason string) {
	c.touch(g1_protocol.ERoleSectionFlag_GUILD_INFO, reason)
}

func guildSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_GUILD_INFO, "guild",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleGuildInfo { return i.GuildInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleGuildInfo) { i.GuildInfo = m },
		func() *g1_protocol.RoleGuildInfo { return new(g1_protocol.RoleGuildInfo) })
}

// 礼包兑换段：无 ERoleSectionFlag（flag=0），仅参与持久化与显式全量同步，
// 不进入 mask 驱动的增量链路（mask&0 恒为 0）。

type GiftComponent struct{ sectionBase }

func NewGiftComponent() *GiftComponent { return &GiftComponent{} }

func (c *GiftComponent) Name() string { return "gift" }

func (c *GiftComponent) Flag() g1_protocol.ERoleSectionFlag { return 0 }

func (c *GiftComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *GiftComponent) OnDestroy()            {}

func (c *GiftComponent) InitField(uid uint64) {}

func (c *GiftComponent) Touch(reason string) {
	// 无段位：仅标记持久化全量写（force 路径覆盖 gift field）。
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ALL, reason)
}

func giftSection() roleSection {
	return messageSection(0, "gift",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleGiftExchangeInfo { return i.GiftInfo },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleGiftExchangeInfo) { i.GiftInfo = m },
		func() *g1_protocol.RoleGiftExchangeInfo { return new(g1_protocol.RoleGiftExchangeInfo) })
}
