package drop_item_config

import (
	protocol "github.com/Iori372552686/g1_common/protocol"
)

// GetByDropId 按"掉落Id"查询掉落包内全部物品条目。
// 一个 DropId 对应多条物品（不同 itemId/概率），返回全部启用项。
// 注：drop_item_config.gen.go 只生成按主键 DropItemId 的索引；
// 掉落业务需要按 DropId 分组查询，故在此辅助文件补充。掉落表数据量小，遍历可接受。
func GetByDropId(dropId int32) []*protocol.DropItemConfig {
	var out []*protocol.DropItemConfig
	Range(func(c *protocol.DropItemConfig) bool {
		if c.DropId == dropId {
			out = append(out, c)
		}
		return true
	})
	return out
}
