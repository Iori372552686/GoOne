package service

import (
	"bytes"
)

func genIndex(buf *bytes.Buffer) error {
	// 复合键容器类型（Index2/3/4）已内置到 module/gamedata/index.go（手写，永久稳定），
	// 不再生成 index.gen.go。此函数保留为 no-op 以维持调用链不变。
	// 如未来需要生成其他索引相关产物，可在此扩展。
	return nil
}
