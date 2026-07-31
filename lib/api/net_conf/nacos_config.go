package net_conf

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// nacos  client config struct
type NacosConf struct {
	IPAddr      string `json:"ip_addr" yaml:"ip_addr"`
	Port        int    `json:"port" yaml:"port"`
	NamespaceID string `json:"namespace_id" yaml:"namespace_id"`
	GroupName   string `json:"group_name" yaml:"group_name"`
	LogDir      string `json:"log_dir" yaml:"log_dir"`
	CacheDir    string `json:"cache_dir" yaml:"cache_dir"`
	RotateTime  string `json:"rotate_time" yaml:"rotate_time"`
	MaxAge      int    `json:"max_age" yaml:"max_age"`
	LogLevel    string `json:"log_level" yaml:"log_level"`
	UserName    string `json:"user_name" yaml:"user_name"`
	Password    string `json:"password" yaml:"password"`
}

// NewNacosConfigClient 构造 Nacos 配置中心 client。
//
// Deprecated: gamedata 远端配置统一经 lib/contrib/config/factory 构造
// 后端无关的 config.Client（见 common/gamedata.InitNacos/InitRemote），监听回收由
// contrib watcher.Stop 负责。本函数保留仅供仍需裸 nacos IConfigClient 的旧代码使用。
// 构造失败返回 error，不返回 nil client。
func NewNacosConfigClient(conf NacosConf) (config_client.IConfigClient, error) {
	//server conf
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(conf.IPAddr, uint64(conf.Port)),
	}

	//client conf
	cc := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		NamespaceId:         conf.NamespaceID,
		LogDir:              conf.LogDir,
		CacheDir:            conf.CacheDir,
		LogLevel:            conf.LogLevel,
		Username:            conf.UserName,
		Password:            conf.Password,
	}

	// a more graceful way to create naming client
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		logger.Errorf("NewConfigClient err | %v", err)
		return nil, fmt.Errorf("nacos: new config client: %w", err)
	}

	return client, nil
}
