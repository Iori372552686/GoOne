package cmd_handler

import (
	"GoOne/lib/api/cmd_handler"
	"GoOne/src/mainsvr/globals"
	"GoOne/src/mainsvr/role"
)

type IRoleCmd interface {
	ProcessCmd(c cmd_handler.IContext, data []byte, myRole *role.Role) int
}

type roleAdapter struct {
	roleCmd IRoleCmd
}

func NewRoleAdapter(roleCmd IRoleCmd) cmd_handler.ICmdHandler {
	a := new(roleAdapter)
	a.roleCmd = roleCmd
	return a
}

func (t *roleAdapter) ProcessCmd(c cmd_handler.IContext, data []byte) int {
	myRole := globals.RoleMgr.GetOrLoadRole(c.Uid(), c)
	if myRole == nil {
		return -1
	}

	myRole.Lock()
	defer myRole.Unlock()

	return t.roleCmd.ProcessCmd(c, data, myRole)
}
