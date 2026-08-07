//go:build config_etcd
// +build config_etcd

package factory

import (
	"context"
	"fmt"
	"strings"

	contribconfig "github.com/Iori372552686/GoOne/lib/contrib/config"
	conf_etcd "github.com/Iori372552686/GoOne/lib/contrib/config/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type etcdClient struct {
	contribconfig.Client
	cli *clientv3.Client
}

func (e *etcdClient) Close() error {
	if e.cli != nil {
		return e.cli.Close()
	}
	return nil
}

// newEtcdClientImpl 构造 etcd client，读/写共用。Username/Password 为空则不带鉴权。
func newEtcdClientImpl(cfg Config) (*clientv3.Client, error) {
	if len(cfg.Addrs) == 0 {
		return nil, fmt.Errorf("etcd: missing address")
	}
	cc := clientv3.Config{
		Endpoints:   cfg.Addrs,
		DialTimeout: cfg.Timeout,
	}
	if cfg.EtcdUserName != "" || cfg.EtcdPassword != "" {
		cc.Username = cfg.EtcdUserName
		cc.Password = cfg.EtcdPassword
	}
	return clientv3.New(cc)
}

func newEtcdClient(cfg Config) (contribconfig.Client, error) {
	cli, err := newEtcdClientImpl(cfg)
	if err != nil {
		return nil, err
	}
	src, err := conf_etcd.New(cli,
		conf_etcd.WithContext(context.Background()),
		conf_etcd.WithPath(cfg.Path),
		conf_etcd.WithPrefix(cfg.EtcdPrefix),
	)
	if err != nil {
		_ = cli.Close()
		return nil, err
	}
	return &etcdClient{Client: contribconfig.Wrap(src, nil), cli: cli}, nil
}

func newEtcdPublisher(cfg Config) (contribconfig.Publisher, error) {
	if strings.TrimSpace(cfg.Path) == "" {
		return nil, fmt.Errorf("etcd: missing path")
	}
	cli, err := newEtcdClientImpl(cfg)
	if err != nil {
		return nil, err
	}
	return conf_etcd.NewPublisher(cli, conf_etcd.WithPubPath(cfg.Path))
}
