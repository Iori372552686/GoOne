package component

import (
	"fmt"
)

// SendGM 发送 GM 指令。GoOne GM 协议为独立 CMD（如 CMD_MAIN_GM_ADD_ITEM_REQ），
// 各字段语义不同，无法像 seed-tester 那样用统一 Cmd/Params 封装。
// 这里保留接口占位，具体 GM 操作请在业务组件内直接构造对应 proto 请求。
func SendGM(_ MessageSender, cmd string, _ ...string) error {
	return fmt.Errorf("SendGM %q not implemented in GoOne tester: use specific GM proto request", cmd)
}
