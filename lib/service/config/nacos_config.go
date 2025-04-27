package config

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

// nacos  client config struct
type NacosConf struct {
	IPAddr      string `json:"ip_addr"`
	Port        int    `json:"port"`
	NamespaceID string `json:"namespace_id"`
	GroupName   string `json:"group_name"`
	LogDir      string `json:"log_dir"`
	CacheDir    string `json:"cache_dir"`
	RotateTime  string `json:"rotate_time"`
	MaxAge      int    `json:"max_age"`
	LogLevel    string `json:"log_level"`
}

func NewNacosConfigClient(conf NacosConf) *config_client.IConfigClient {
	//server config
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(conf.IPAddr, uint64(conf.Port)),
	}

	//client config
	cc := constant.ClientConfig{
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		NamespaceId:         conf.NamespaceID,
		LogDir:              conf.LogDir,
		CacheDir:            conf.CacheDir,
		LogLevel:            conf.LogLevel,
	}

	// a more graceful way to create naming client
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		logger.Infof("NewConfigClient err | ", err.Error())
	}

	return &client
}
