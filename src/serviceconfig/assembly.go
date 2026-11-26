// Package serviceconfig connects GoOne's configuration to the reusable bus-service components.
package serviceconfig

import (
	"github.com/Iori372552686/GoOne/lib/service/runtime/bussvc"
	"github.com/Iori372552686/GoOne/lib/service/ssrpc"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/misc"
)

// ForGoOne returns the configuration callbacks for GoOne's service assembly.
// Configuration is loaded and validated at startup, not during construction.
func ForGoOne() bussvc.Assembly {
	return bussvc.Assembly{
		LoadServiceConfig: loadServiceConfig,
		CommonForService:  commonForService,
		RouteRules:        func() map[uint32]uint32 { return misc.ServerRouteRules },
	}
}

func loadServiceConfig(service string) error {
	if err := conf.Load(conf.Path); err != nil {
		return err
	}
	return conf.RunValidators(service)
}

func commonForService(svc string) bussvc.Common {
	admPrefix := "base_cfg.runtime.admin_server."
	admPort := conf.ResolveAdminPort(conf.Get(admPrefix+"port").Int(), svc)
	trPrefix := "base_cfg.runtime.tracing."
	return bussvc.Common{
		LogDir:       conf.Get(svc + ".debug.log_dir").String(),
		LogLevel:     conf.Get(svc + ".debug.log_level").String(),
		SelfBusId:    conf.Get(svc + ".identity.self_bus_id").String(),
		BusMQAddr:    conf.Get("base_cfg.runtime.bus_mq_addr").String(),
		RegisterAddr: conf.Get("base_cfg.runtime.register_addr").String(),
		AdminEnabled: conf.Get(admPrefix + "enabled").Bool(),
		AdminIP:      conf.Get(admPrefix + "ip").String(),
		AdminPort:    admPort,
		Pprof:        conf.Get("base_cfg.debug.pprof").Bool(),
		Tracing: ssrpc.TracingConfig{
			Enabled:      conf.Get(trPrefix + "enabled").Bool(),
			Exporter:     conf.Get(trPrefix + "exporter").String(),
			Endpoint:     conf.Get(trPrefix + "endpoint").String(),
			Insecure:     conf.Get(trPrefix + "insecure").Bool(),
			SamplerRatio: conf.Get(trPrefix + "sampler_ratio").Float64(),
			Headers:      conf.Get(trPrefix + "headers").StringsMap(),
		},
	}
}
