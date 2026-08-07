package config

import "context"

// Publisher 与 Source（只读 Load/Watch）对称，面向"写"：把一个数据产物
// （dataID + 内容 + 格式）发布到配置中心。dataID 由调用方决定，通常是带后缀
// 的文件名，如 "ItemConfig.json"；format 为格式提示（"json"/"text"/"bytes"/"yaml" 等），
// 后端按需透传（Nacos 写入 Type 字段，etcd 仅作为元信息不影响存储）。
//
// 与 Source 一样，Publisher 也是一个后端无关的本地抽象，具体实现由
// lib/contrib/config/<backend> 提供，统一通过 factory.NewPublisher / NewPublisherFromURL 构造。
type Publisher interface {
	// Publish 把 content 以 dataID 为键发布到配置中心。
	// content 为原始字节；format 为格式提示，部分后端（如 Nacos）会写入配置类型字段。
	Publish(ctx context.Context, dataID string, content []byte, format string) error
	Close() error
}
