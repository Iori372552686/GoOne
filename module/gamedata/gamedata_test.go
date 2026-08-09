package gamedata

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain 在所有 gamedata 测试前初始化本地配置加载。
// data 目录位于本包目录下，用绝对路径定位避免依赖 cwd。
func TestMain(m *testing.M) {
	dataDir := filepath.Join(".", "data")
	if _, err := os.Stat(dataDir); err != nil {
		// data 目录不存在时跳过初始化（某些 CI 环境可能不 checkout 数据文件）
		os.Exit(m.Run())
	}
	if err := InitLocal(dataDir); err != nil {
		panic(err)
	}
	m.Run()
}
