//go:build !config_etcd
// +build !config_etcd

package factory

import (
	"fmt"

	contribconfig "github.com/Iori372552686/GoOne/lib/contrib/config"
)

func newEtcdClient(cfg Config) (contribconfig.Client, error) {
	_ = cfg
	return nil, fmt.Errorf("etcd config backend not enabled (build with: -tags config_etcd)")
}

func newEtcdPublisher(cfg Config) (contribconfig.Publisher, error) {
	_ = cfg
	return nil, fmt.Errorf("etcd config backend not enabled (build with: -tags config_etcd)")
}
