package role

import (
	"reflect"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"google.golang.org/protobuf/proto"
)

// ============================================================
// 玩家功能组件体系
//
// RoleComponent        逻辑型组件（无数据段，如掉落/获得展示/道具使用）
// SectionComponent     数据型组件：拥有 RoleInfo 的一个数据段
// PatchComponent       增量型组件：map 型大数据段按 key 维护脏集
//
// 组件均无锁——与 Role 方法一致，全部运行在 TransMgr 的 uid 串行域内。
// 命名避开框架层 runtime.Component 与 tester.TesterComponent。
// ============================================================

// RoleComponent 玩家功能组件基础契约。
type RoleComponent interface {
	// Name 组件名。数据型组件的 Name 同时是同步段名与 Redis hash field 名（单一来源）。
	Name() string
	OnInit(r *Role) error
	OnDestroy()
}

// SectionComponent 数据型组件：绑定 RoleInfo 的一个数据段。
type SectionComponent interface {
	RoleComponent
	// Flag 段位。无 ERoleSectionFlag 的段（如 gift）返回 0——只参与持久化，
	// 不参与 mask 驱动的同步（mask&0 恒为 0，天然排除，含 ALL=-1）。
	Flag() g1_protocol.ERoleSectionFlag
	// InitField 对应数据段的 nil 兜底初始化（原 RoleInitField 的逐段拆分）。
	InitField(uid uint64)
	// Touch 全量档脏标记：客户端全量同步 + 持久化段脏。
	Touch(reason string)
}

// PatchComponent 增量型组件：map 型大数据段按 key 维护脏集，
// FlushClientSync 时只下发变更 key 的 patch。
type PatchComponent interface {
	SectionComponent
	MarkDirty(key int32, deleted bool)
	// BuildPatch 把变更集写入 v2 同步包；无变更返回 false。
	BuildPatch(dst *g1_protocol.ScSyncUserDataV2) bool
	ClearDirty()
}

// OnlineComponent 可选钩子：OnLogin 时机（原 r.OnLogin 的分段扩展点）。
type OnlineComponent interface {
	Online() error
}

// DailyResetComponent 可选钩子：everyDayCheck 时机。
type DailyResetComponent interface {
	DailyReset() error
}

// L3Critical 高频写段标记：实现并返回 true 时，MaybeFlushL3 无视防抖立即
// 投递 L3 快照（货币等价值敏感段；L2 是最新数据唯一副本的风险缓解）。
type L3Critical interface {
	RequireImmediateL3() bool
}

// L3FlushObserver L3 快照投递成功后的通知钩子（清除 L3Critical 待写标记）。
type L3FlushObserver interface {
	OnL3Flushed()
}

// ============================================================
// 数据段静态描述注册表
//
// 一个数据段在 持久化 marshal / 持久化 unmarshal / 同步填充 三条链路上的
// 全部静态行为收敛为一条 roleSection 记录，由各组件文件就近提供、此处
// 按序装配——取代旧的 roleSectionDefs / roleSectionAccessors /
// unmarshalSection 三处平行认知。新增数据段 = 写组件文件 + 在
// buildRoleSections 加一行，不再触碰 sync_state / persist_hash / store。
// ============================================================

// roleSection 一个 RoleInfo 数据段的静态描述。
type roleSection struct {
	flag g1_protocol.ERoleSectionFlag
	name string
	// getMsg 取段消息用于 marshal 与同步（可能为 nil，nil 跳过写入）。
	getMsg func(info *g1_protocol.RoleInfo) proto.Message
	// setMsg 同步填充：把段消息赋给目标 RoleInfo 的对应字段。
	setMsg func(info *g1_protocol.RoleInfo, msg proto.Message)
	// unmarshal 持久化加载：反序列化并赋回 RoleInfo 对应字段。
	unmarshal func(info *g1_protocol.RoleInfo, buf []byte) error
}

// messageSection 段描述的泛型构造，消除每个 section 的手写样板。
// get/set/newMsg 三闭包由各组件文件用具体 pb 类型提供。
//
// getMsg 对未设置的段返回 untyped nil（具体指针为 nil 时）——避免 typed-nil
// 接口穿透 nil 检查、把空消息当作有效段写入（空段写出的 0 字节 field 属于
// 历史遗留 wart，加载侧本就跳过空 buf，此语义对存量数据完全兼容）。
func messageSection[PT proto.Message](
	flag g1_protocol.ERoleSectionFlag,
	name string,
	get func(info *g1_protocol.RoleInfo) PT,
	set func(info *g1_protocol.RoleInfo, msg PT),
	newMsg func() PT,
) roleSection {
	return roleSection{
		flag: flag,
		name: name,
		getMsg: func(info *g1_protocol.RoleInfo) proto.Message {
			m := get(info)
			if isNilProtoMessage(m) {
				return nil
			}
			return m
		},
		setMsg: func(info *g1_protocol.RoleInfo, msg proto.Message) {
			set(info, msg.(PT))
		},
		unmarshal: func(info *g1_protocol.RoleInfo, buf []byte) error {
			m := newMsg()
			if err := proto.Unmarshal(buf, m); err != nil {
				return err
			}
			set(info, m)
			return nil
		},
	}
}

// isNilProtoMessage 具体指针为 nil 的 proto 消息判定（typed-nil 接口剥离）。
func isNilProtoMessage(m proto.Message) bool {
	if m == nil {
		return true
	}
	rv := reflect.ValueOf(m)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

// roleSectionRegistry 全部数据段的唯一注册表（顺序即装配顺序）。
var roleSectionRegistry = buildRoleSections()

func buildRoleSections() []roleSection {
	return []roleSection{
		registerSection(),
		loginSection(),
		gameSection(),
		basicSection(),
		currencySection(),
		inventorySection(),
		iconSection(),
		mallSection(),
		mainTaskSection(),
		guildSection(),
		guideSection(),
		openFuncSection(),
		activityTaskSection(),
		giftSection(),
	}
}

// sectionByName 按段名（hash field 名）查找；找不到返回 nil。
func sectionByName(name string) *roleSection {
	for i := range roleSectionRegistry {
		if roleSectionRegistry[i].name == name {
			return &roleSectionRegistry[i]
		}
	}
	return nil
}

// sectionByFlag 按段位查找；flag=0 或未注册返回 nil。
func sectionByFlag(flag g1_protocol.ERoleSectionFlag) *roleSection {
	if flag == 0 {
		return nil
	}
	for i := range roleSectionRegistry {
		if roleSectionRegistry[i].flag == flag {
			return &roleSectionRegistry[i]
		}
	}
	return nil
}

// marshalSectionsByMask 依据脏 mask 产出待写段名集合；mask 含 ALL 或
// force 语义由调用方归一为 ALL 后传 sections=nil（codec 约定 nil=全量）。
func marshalSectionsByMask(mask g1_protocol.ERoleSectionFlag) []string {
	if mask == g1_protocol.ERoleSectionFlag_ALL {
		return nil
	}
	var sections []string
	for i := range roleSectionRegistry {
		sec := &roleSectionRegistry[i]
		if sec.flag != 0 && hasRoleSection(mask, sec.flag) {
			sections = append(sections, sec.name)
		}
	}
	return sections
}

// sectionBase 数据型组件的公共骨架：持有宿主 Role 并提供 Touch 默认实现。
type sectionBase struct {
	role *Role
}

func (b *sectionBase) bind(r *Role) { b.role = r }

func (b *sectionBase) touch(flag g1_protocol.ERoleSectionFlag, reason string) {
	b.role.MarkFullSync(flag)
	b.role.markPersistSectionDirty(flag, reason)
}
