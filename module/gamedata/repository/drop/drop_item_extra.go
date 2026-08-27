package drop

import (
	protocol "github.com/Iori372552686/g1_common/protocol"
)

// GetDropItemByDropId 按"掉落Id"查询掉落包内全部物品条目。
func GetDropItemByDropId(dropId int32) []*protocol.DropItemConfig {
	var out []*protocol.DropItemConfig
	RangeDropItem(func(c *protocol.DropItemConfig) bool {
		if c.DropId == dropId {
			out = append(out, c)
		}
		return true
	})
	return out
}
