package test

import (
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
)

// 本文件覆盖今天的命名体系改动（naming.go）：
//   1. ParseFileFeature：xlsx 文件名 @ 拆分功能名/中文名
//   2. ParseSheetTable：sheet 名 @ 拆分表名/中文名
//   3. ConfigTypeName：功能名+表名+Config，同名去重
//   4. GoFileName / GoPkgName / ProtoFileName：产物命名
//   5. 无 @ 退化为旧行为

func TestParseFileFeature(t *testing.T) {
	cases := []struct {
		name        string
		wantFeature string
		wantChinese string
	}{
		{"掉落系统@drop.xlsx", "drop", "掉落系统"},
		{"道具@item.xlsx", "item", "道具"},
		{"德州@texas.xlsx", "texas", "德州"},
		// 无 @ 退化：功能名=文件基名，中文名为空
		{"Const.xlsx", "Const", ""},
		{"enum.xlsx", "enum", ""},
		// 边界：@ 在开头（@ 前为空）不拆分，基名原样返回
		{"@drop.xlsx", "@drop", ""},
	}
	for _, c := range cases {
		feat, chn := base.ParseFileFeature(c.name)
		if feat != c.wantFeature || chn != c.wantChinese {
			t.Errorf("ParseFileFeature(%q) = (%q, %q), want (%q, %q)",
				c.name, feat, chn, c.wantFeature, c.wantChinese)
		}
	}
}

func TestParseSheetTable(t *testing.T) {
	cases := []struct {
		name     string
		wantTbl  string
		wantChn  string
	}{
		{"掉落组表@group", "group", "掉落组表"},
		{"掉落表@item", "item", "掉落表"},
		{"状态机表@machine", "machine", "状态机表"},
		// 无 @ 退化：表名=sheet 名本身
		{"Data", "Data", ""},
		{"全局常量表", "全局常量表", ""},
	}
	for _, c := range cases {
		tbl, chn := base.ParseSheetTable(c.name)
		if tbl != c.wantTbl || chn != c.wantChn {
			t.Errorf("ParseSheetTable(%q) = (%q, %q), want (%q, %q)",
				c.name, tbl, chn, c.wantTbl, c.wantChn)
		}
	}
}

func TestConfigTypeName(t *testing.T) {
	cases := []struct {
		feature string
		table   string
		want    string
	}{
		{"drop", "group", "DropGroupConfig"},
		{"drop", "item", "DropItemConfig"},
		{"item", "use", "ItemUseConfig"},
		{"obtain", "policy", "ObtainPolicyConfig"},
		{"texas", "machine", "TexasMachineConfig"},
		{"mall", "mall", "MallConfig"},             // 同名去重
		{"decompose", "decompose", "DecomposeConfig"}, // 同名去重
		{"item", "item", "ItemConfig"},             // 同名去重
		// 复合词表名（rule_ref -> RuleRef）
		{"item", "rule_ref", "ItemRuleRefConfig"},
		{"item", "rule", "ItemRuleConfig"},
	}
	for _, c := range cases {
		got := base.ConfigTypeName(c.feature, c.table)
		if got != c.want {
			t.Errorf("ConfigTypeName(%q, %q) = %q, want %q", c.feature, c.table, got, c.want)
		}
	}
}

func TestGoFileName(t *testing.T) {
	cases := []struct {
		feature string
		table   string
		want    string
	}{
		{"drop", "group", "gdata_drop_group.go"},
		{"drop", "item", "gdata_drop_item.go"},
		{"item", "rule_ref", "gdata_item_rule_ref.go"},
		{"consts", "global", "gdata_consts_global.go"},
		{"mall", "mall", "gdata_mall_mall.go"},
	}
	for _, c := range cases {
		got := base.GoFileName(c.feature, c.table)
		if got != c.want {
			t.Errorf("GoFileName(%q, %q) = %q, want %q", c.feature, c.table, got, c.want)
		}
	}
}

func TestGoPkgName(t *testing.T) {
	cases := []struct {
		feature string
		want    string
	}{
		{"drop", "drop"},
		{"DropItem", "drop_item"}, // 无 @ 退化文件名的 snake 化
		{"consts", "consts"},
		{"itemrule", "itemrule"},
	}
	for _, c := range cases {
		got := base.GoPkgName(c.feature)
		if got != c.want {
			t.Errorf("GoPkgName(%q) = %q, want %q", c.feature, got, c.want)
		}
	}
}

func TestProtoFileName(t *testing.T) {
	if got := base.ProtoFileName("drop"); got != "drop" {
		t.Errorf("ProtoFileName(drop) = %q, want drop", got)
	}
	if got := base.ProtoFileName("texas"); got != "texas" {
		t.Errorf("ProtoFileName(texas) = %q, want texas", got)
	}
}
