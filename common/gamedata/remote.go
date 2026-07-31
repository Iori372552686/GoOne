package gamedata

import (
	"fmt"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/api/uerror"
	"github.com/Iori372552686/GoOne/lib/contrib/config/factory"
)

// InitNacos 在 app init 时基于 lib/contrib/config/factory 构造 Nacos 后端 client，
// 并接管 gamedata 的远端加载与热更（V4 P0-06）。
//
// 设计要点：
//   - gamedata 核心只面向 contrib config.Client 抽象（见 InitRemote），后端由 factory
//     统一构造；本函数仅是 NacosConf → factory.Config 的适配，便于后续切换 etcd/consul
//     等后端时只改本文件；
//   - DataIDs 取自 SheetFiles()（已注册的表，字典序）；
//   - 失败时关闭已构造的 client，不留半启动状态；成功后由 StopNet 统一回收。
func InitNacos(conf net_conf.NacosConf) error {
	dataIDs := SheetFiles()
	if len(dataIDs) == 0 {
		return uerror.New(1, -1, "gamedata.InitNacos: no sheets registered")
	}

	cli, err := factory.NewClient(factory.Config{
		Backend:          factory.BackendNacos,
		Addrs:            []string{fmt.Sprintf("%s:%d", conf.IPAddr, conf.Port)},
		Timeout:          5 * time.Second, // 与旧 net_conf.NewNacosConfigClient 的 TimeoutMs=5000 对齐
		NacosDataIDs:     dataIDs,
		NacosGroup:       conf.GroupName,
		NacosNamespaceID: conf.NamespaceID,
		NacosUserName:    conf.UserName,
		NacosPassword:    conf.Password,
		NacosLogDir:      conf.LogDir,
		NacosCacheDir:    conf.CacheDir,
		NacosLogLevel:    conf.LogLevel,
	})
	if err != nil {
		return err
	}
	if err := InitRemote(cli); err != nil {
		_ = cli.Close()
		return err
	}
	return nil
}
