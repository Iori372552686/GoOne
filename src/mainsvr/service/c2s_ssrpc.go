package service

import (
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
)

// MainC2SServiceImpl is the IDL-driven ssrpc implementation for mainsvr client commands.
//
// 文件按用例拆分（报告 §7.8）：handler 只做协议转换，业务语义收敛在 role/ 与
// room/ 包的用例方法内。
//   - c2s_login.go      登录 / 退出 / 心跳
//   - c2s_profile.go    改名 / 头像装扮
//   - c2s_gm.go         GM 查询 / 设置 / 发道具
//   - c2s_inventory.go  背包道具
//   - c2s_mall.go       商城购买
//   - c2s_room.go       房间与对局转发
type MainC2SServiceImpl struct {
	// roles 是玩家会话用例（加载 / 登出 / 过期淘汰）。经构造注入替代 handler
	// 内的包级 globals 访问（报告 §7.1 试点）：同进程可构造多个互不污染的服务
	// 实例，测试无需重置全局变量。
	roles *role.RoleMgr
}

// NewMainC2SServiceImpl 注入实际依赖。roles 为 nil 时回退全局 RoleMgr，兼容
// 旧装配路径；生产装配显式传入 globals.RoleMgr。
func NewMainC2SServiceImpl(roles *role.RoleMgr) *MainC2SServiceImpl {
	if roles == nil {
		roles = globals.RoleMgr
	}
	return &MainC2SServiceImpl{roles: roles}
}
