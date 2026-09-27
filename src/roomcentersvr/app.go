package roomcentersvr

import (
	"context"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/api/net_conf"
	"github.com/Iori372552686/GoOne/lib/service/bus/driver/rabbitmq"
	"github.com/Iori372552686/GoOne/lib/service/router"
	"github.com/Iori372552686/GoOne/lib/service/runtime"
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/gamedata"
	"github.com/Iori372552686/GoOne/src/roomcentersvr/globals"
)

// NewApp 用 runtime.App + Component 装配 roomcentersvr。服务名只在 MustNew 出现一次；
// 标准组件（logger/admin/tracing/router）由 bussvc 构造器自读 conf 装配。
//
// 旧德州项目的房间目录业务（room_mgr/texas_room、room_ai、RoomCenterInnerService）
// 已于 2026-09-27 移除；当前仅保留总线服务骨架，待新游戏的房间系统在此落位。
func NewApp() *runtime.App {
	// gamedata 本地目录加载作为 LoadConfig 的追加钩子，在校验通过后执行。
	app := bussvc.MustNew("roomcentersvr", router.ReadyCheck, bussvc.WithConfLoader(func(_ context.Context) error {
		if gameDataDir := conf.Get("base_cfg.dependencies.game_data_dir").String(); gameDataDir != "" {
			logger.Infof("Loading local file by gameconf_dir: %v ", gameDataDir)
			if err := gamedata.InitLocal(gameDataDir); err != nil {
				return err
			}
		}
		return nil
	}))

	transMgr := &bussvc.TransMgrComponent{
		Mgr: globals.TransMgr,
	}

	businessDeps := &bussvc.FuncComponent{
		ComponentName: "business_deps",
		OnStart: func(ctx context.Context) error {
			var nacosConf net_conf.NacosConf
			_ = conf.Unmarshal("base_cfg.dependencies.nacos_conf", &nacosConf)
			if nacosConf.IPAddr != "" {
				logger.Infof("Loading remote gameconf by Nacos group: %v ", nacosConf.GroupName)
				// 经 lib/contrib/config/factory 构造配置中心 client，
				// 由 gamedata.InitRemote 统一加载+热更；构造/拉取失败返回 error。
				if err := gamedata.InitNacos(nacosConf); err != nil {
					return err
				}
			}
			return nil
		},
		OnStop: func(_ context.Context) error {
			gamedata.StopNet()
			return nil
		},
	}

	routerComp := bussvc.NewRouterComponent(app, globals.TransMgr, rabbitmq.NewRegistry())

	app.MustRegister(
		businessDeps, transMgr, routerComp,
	)
	return app
}
