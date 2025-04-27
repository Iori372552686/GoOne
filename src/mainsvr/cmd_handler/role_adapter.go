package cmd_handler

import (
	"github.com/Iori372552686/GoOne/lib/api/cmd_handler"
	"github.com/Iori372552686/GoOne/src/mainsvr/globals"
	"github.com/Iori372552686/GoOne/src/mainsvr/role"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

type IRoleCmd func(c cmd_handler.IContext, data []byte, myRole *role.Role) g1_protocol.ErrorCode

type roleAdapter struct {
	roleCmd IRoleCmd
}

func NewRoleAdapter(roleCmd IRoleCmd) cmd_handler.CmdHandlerFunc {
	a := new(roleAdapter)
	a.roleCmd = roleCmd
	return a.ProcessCmd
}

func (t *roleAdapter) ProcessCmd(c cmd_handler.IContext, data []byte) g1_protocol.ErrorCode {
	myRole := globals.RoleMgr.GetOrLoadRole(c.Uid(), c)
	if myRole == nil {
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	//myRole.Lock()  不加也行
	//defer myRole.Unlock()
	return t.roleCmd(c, data, myRole)
}
