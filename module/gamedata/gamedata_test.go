package gamedata

import (
	"os"
	"testing"
)

// TestMain 在所有 gamedata 测试前初始化本地配置加载。
//
// 数据文件（.conf）已迁移到 common/game_data/（g1_common submodule），
// 不再位于本包目录下。测试时通过环境变量 GOONE_GAME_DATA_DIR 指定路径，
// 未设置则跳过本地初始化（仅跑不依赖配置数据的测试，如 remote mock 测试）。
func TestMain(m *testing.M) {
	dataDir := os.Getenv("GOONE_GAME_DATA_DIR")
	if dataDir == "" {
		// 回退：尝试相对路径（假设从仓库根跑 go test）
		candidates := []string{
			"./common/game_data",
			"../common/game_data",
			"../../common/game_data",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				dataDir = c
				break
			}
		}
	}
	if dataDir != "" {
		if err := InitLocal(dataDir); err != nil {
			panic(err)
		}
	}
	m.Run()
}
