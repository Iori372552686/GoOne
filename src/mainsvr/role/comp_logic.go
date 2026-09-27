package role

// 逻辑型组件：不拥有 RoleInfo 数据段，仅纳入统一组件注册与生命周期。
// 业务算法仍以 *Role 方法形态留在各自文件（drop_group.go / drop.go /
// item.go 的道具使用分发），组件壳为后续生命周期钩子与依赖声明预留落点。

// DropComponent 掉落系统组件（双层解析算法见 drop_group.go / drop.go）。
type DropComponent struct {
	role *Role
}

func NewDropComponent() *DropComponent { return &DropComponent{} }

func (c *DropComponent) Name() string         { return "drop" }
func (c *DropComponent) OnInit(r *Role) error { c.role = r; return nil }
func (c *DropComponent) OnDestroy()           {}

// ItemUseComponent 道具使用分发组件（UseType 分发见 item.go）。
type ItemUseComponent struct {
	role *Role
}

func NewItemUseComponent() *ItemUseComponent { return &ItemUseComponent{} }

func (c *ItemUseComponent) Name() string         { return "itemuse" }
func (c *ItemUseComponent) OnInit(r *Role) error { c.role = r; return nil }
func (c *ItemUseComponent) OnDestroy()           {}
