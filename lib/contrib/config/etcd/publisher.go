package etcd

import (
	"context"
	"errors"
	"path"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// pubOption 与 source 的 Option 对称；发布时只需 path 前缀。
type pubOption func(o *pubOptions)

type pubOptions struct {
	path string
}

// WithPubPath 设置发布的 key 前缀（每个 dataID 拼接为 path.Join(prefix, dataID)）。
func WithPubPath(p string) pubOption {
	return func(o *pubOptions) { o.path = p }
}

// Publisher 是 etcd 的写实现：调用 clientv3.Put 发布配置。
//
// 与 Nacos 不同，etcd 的 value 是原始字节，因此 .bytes 这类二进制产物可二进制安全地存储。
type Publisher struct {
	cli  *clientv3.Client
	opts *pubOptions
}

// NewPublisher 构造一个 etcd 写 Publisher。
func NewPublisher(cli *clientv3.Client, opts ...pubOption) (*Publisher, error) {
	if cli == nil {
		return nil, errors.New("etcd client is nil")
	}
	o := &pubOptions{}
	for _, opt := range opts {
		opt(o)
	}
	o.path = strings.TrimSpace(o.path)
	if o.path == "" {
		return nil, errors.New("path invalid")
	}
	return &Publisher{cli: cli, opts: o}, nil
}

// Publish 把 content 以 dataID 发布到 etcd，key 为 path.Join(prefix, dataID)。
func (p *Publisher) Publish(ctx context.Context, dataID string, content []byte, format string) error {
	_ = format // etcd 不区分配置类型
	dataID = strings.TrimSpace(dataID)
	if dataID == "" {
		return errors.New("etcd publish: dataID is empty")
	}
	key := path.Join(p.opts.path, dataID)
	_, err := p.cli.Put(ctx, key, string(content))
	return err
}

// Close 关闭底层 etcd client。由调用方在发布结束后负责（通常一次发布即退出）。
func (p *Publisher) Close() error {
	if p.cli != nil {
		return p.cli.Close()
	}
	return nil
}
