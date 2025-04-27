package plug

// CmdBlacklist 管理int32类型命令的黑名单,需要注意并发安全，无锁
type CmdBlacklist struct {
	commands map[uint32]bool
}

func NewCmdBlacklist() *CmdBlacklist {
	return &CmdBlacklist{
		commands: make(map[uint32]bool),
	}
}

func (c *CmdBlacklist) Register(cmd uint32) {
	c.commands[cmd] = true
}

func (c *CmdBlacklist) IsBlocked(cmd uint32) bool {
	return c.commands[cmd]
}

func (c *CmdBlacklist) Unregister(cmd uint32) {
	delete(c.commands, cmd)
}

// GetAll 获取所有黑名单命令
func (c *CmdBlacklist) GetAll() []uint32 {
	list := make([]uint32, 0, len(c.commands))
	for cmd := range c.commands {
		list = append(list, cmd)
	}
	return list
}
