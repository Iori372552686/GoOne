package nacos

import (
	"context"
	"errors"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// pubOption 与 source 的 Option 对称：group 是发布时唯一可调项；
// dataID 在 Publish 时按条目给出（不同于 source 需要预先固定 dataID 列表）。
type pubOption func(o *pubOptions)

type pubOptions struct {
	group string
}

// WithPubGroup 设置发布使用的 group（默认 DEFAULT_GROUP），与 source.WithGroup 对齐。
func WithPubGroup(group string) pubOption {
	return func(o *pubOptions) { o.group = group }
}

// Publisher 是 nacos 的写实现：调用 config_client.PublishConfig 发布配置。
//
// 注意：Nacos 以文本字符串存储配置内容（vo.ConfigParam.Content 为 string），
// 因此 .bytes 这类二进制产物以 string(content) 透传时，非 UTF-8 字节可能被
// 服务端/客户端按平台处理而失真；文本类产物（json/conf/lua）无此问题。
type Publisher struct {
	client config_client.IConfigClient
	opts   *pubOptions
}

// NewPublisher 构造一个 nacos 写 Publisher。
func NewPublisher(client config_client.IConfigClient, opts ...pubOption) (*Publisher, error) {
	if client == nil {
		return nil, errors.New("nacos client is nil")
	}
	o := &pubOptions{group: "DEFAULT_GROUP"}
	for _, opt := range opts {
		opt(o)
	}
	o.group = strings.TrimSpace(o.group)
	if o.group == "" {
		o.group = "DEFAULT_GROUP"
	}
	return &Publisher{client: client, opts: o}, nil
}

// Publish 把 content 以 dataID 发布到 nacos。
// format 透传到 vo.ConfigParam.Type（"json"/"text"/"yaml" 等），空则省略。
func (p *Publisher) Publish(ctx context.Context, dataID string, content []byte, format string) error {
	dataID = strings.TrimSpace(dataID)
	if dataID == "" {
		return errors.New("nacos publish: dataID is empty")
	}
	// nacos SDK 目前未把 ctx 透传到底层 HTTP 调用，这里只做边界检查保持接口对称。
	_ = ctx
	param := vo.ConfigParam{
		DataId:  dataID,
		Group:   p.opts.group,
		Content: string(content),
	}
	if f := strings.TrimSpace(format); f != "" {
		param.Type = f
	}
	ok, err := p.client.PublishConfig(param)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("nacos publish: PublishConfig returned false")
	}
	return nil
}

// Close 目前 nacos config client 未暴露可靠的 Close，保持 no-op（与 factory 中 nacos 注释一致）。
func (p *Publisher) Close() error { return nil }
