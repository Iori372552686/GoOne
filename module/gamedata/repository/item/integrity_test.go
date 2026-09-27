package item

import (
	"os"
	"testing"

	"github.com/Iori372552686/GoOne/module/gamedata"
)

// TestItemConfigIntegrity 配置关联完整性（代码测验·维度3，P1）：
// ItemRuleRef 的每个 ItemId 须存在于 ItemConfig、每个 RuleId 须存在于
// ItemRuleConfig——悬挂引用使 getItemRule 返回 nil、MaxOwnCount 上限静默失效
//（模拟测试 T13 已暴露过该类后果）。须在仓库根运行（go test ./...）。
func TestItemConfigIntegrity(t *testing.T) {
	if _, err := os.Stat("../../../../common/game_data"); err != nil {
		t.Skip("gamedata 目录不可达（须在仓库根运行）")
	}
	if err := gamedata.InitLocal("../../../../common/game_data"); err != nil {
		t.Fatalf("InitLocal: %v", err)
	}

	items := map[int32]bool{}
	for _, it := range GetItemAll() {
		items[it.GetItemId()] = true
	}
	rules := map[int32]bool{}
	for _, r := range GetItemRuleAll() {
		rules[r.GetId()] = true
	}
	refs := GetItemRuleRefAll()
	if len(items) == 0 || len(refs) == 0 || len(rules) == 0 {
		t.Fatalf("配置为空: items=%d refs=%d rules=%d", len(items), len(refs), len(rules))
	}
	for _, ref := range refs {
		if ref.GetItemId() == 0 || ref.GetRuleId() == 0 {
			t.Errorf("ItemRuleRef 缺 ItemId/RuleId: %+v", ref)
			continue
		}
		if !items[ref.GetItemId()] {
			t.Errorf("悬挂引用: ItemRuleRef.ItemId=%d 不在 ItemConfig", ref.GetItemId())
		}
		if !rules[ref.GetRuleId()] {
			t.Errorf("悬挂引用: ItemRuleRef.ItemId=%d → RuleId=%d 不在 ItemRuleConfig", ref.GetItemId(), ref.GetRuleId())
		}
	}
}

// TestItemConfigNumericBounds 数值边界：ItemId 正数；MainType/SubType/Sale 非负。
func TestItemConfigNumericBounds(t *testing.T) {
	if _, err := os.Stat("../../../../common/game_data"); err != nil {
		t.Skip("gamedata 目录不可达")
	}
	if err := gamedata.InitLocal("../../../../common/game_data"); err != nil {
		t.Fatalf("InitLocal: %v", err)
	}
	for _, it := range GetItemAll() {
		if it.GetItemId() <= 0 {
			t.Errorf("ItemId=%d 非正", it.GetItemId())
		}
		if it.GetMainType() < 0 || it.GetSubType() < 0 || it.GetSale() < 0 {
			t.Errorf("ItemId=%d 数值为负: main=%d sub=%d sale=%d",
				it.GetItemId(), it.GetMainType(), it.GetSubType(), it.GetSale())
		}
	}
}
