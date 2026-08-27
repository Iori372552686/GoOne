package factory

import (
	"github.com/Iori372552686/GoOne/lib/contrib/registry"
	reg_etcd "github.com/Iori372552686/GoOne/lib/contrib/registry/etcd"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type etcdClient struct {
	*reg_etcd.Registry
	cli *clientv3.Client
}

func (e *etcdClient) Close() error {
	if e.cli != nil {
		return e.cli.Close()
	}
	return nil
}

func newEtcdClient(cfg Config) (registry.Client, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Addrs,
		DialTimeout: cfg.Timeout,
	})
	if err != nil {
		return nil, err
	}
	r := reg_etcd.New(cli, reg_etcd.Namespace(cfg.Namespace))
	return &etcdClient{Registry: r, cli: cli}, nil
}