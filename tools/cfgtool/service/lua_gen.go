package service

import (
	"fmt"
	"strings"
)

const luaConfTpl = `
/*
* 本Lua配置由xlsx工具生成，请勿手动修改
*/

--- @module {{configName}}
function getVer()
    return "{{version}}"
end

{{configName}}.Data = {{configData}}

function main()
    return {{configName}}
end
`

// MarshalToLuaConf 将数据转换为 Lua 配置格式
func MarshalToLuaConf(data interface{}, luaRules string) ([]byte, error) {
	luaStr := toLuaTable(data)

	if luaRules != "" {
		strs := strings.Split(luaRules, ":")
		rules := strings.Split(strs[1], ",")
		genStr := luaConfTpl

		if len(rules) >= 1 && strs[0] == "lua" {
			genStr = strings.ReplaceAll(genStr, "{{configName}}", rules[0])
			if len(rules) >= 2 {
				genStr = strings.ReplaceAll(genStr, "{{version}}", rules[1])
			}
		}

		genStr = strings.ReplaceAll(genStr, "{{configData}}", luaStr)
		return []byte(genStr), nil
	} else {
		return []byte("return " + luaStr), nil
	}
}

// 递归将 map 数据转为 Lua table 字符串
func toLuaTable(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		parts := make([]string, 0, len(val))
		for k, v2 := range val {
			parts = append(parts, fmt.Sprintf("[%q]=%s", k, toLuaTable(v2)))
		}
		return "{" + strings.Join(parts, ",") + "}\n"
	case map[interface{}]interface{}:
		// 兜底：非字符串 key 的 map（key 统一 fmt 成字符串以兼容 Lua）
		parts := make([]string, 0, len(val))
		for k, v2 := range val {
			parts = append(parts, fmt.Sprintf("[%q]=%s", fmt.Sprintf("%v", k), toLuaTable(v2)))
		}
		return "{" + strings.Join(parts, ",") + "}\n"
	case []interface{}:
		parts := make([]string, len(val))
		for i, v2 := range val {
			parts[i] = toLuaTable(v2)
		}
		return "{" + strings.Join(parts, ",") + "}"
	case string:
		return fmt.Sprintf("%q", val)
	case float64, int, int32, int64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "nil"
	default:
		return fmt.Sprintf("%q", val)
	}
}
