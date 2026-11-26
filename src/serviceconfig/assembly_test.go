package serviceconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/module/misc"
)

func TestForGoOneConfigurationCallbacks(t *testing.T) {
	conf.ResetForTest()
	oldPath := conf.Path
	t.Cleanup(func() {
		conf.Path = oldPath
		conf.ResetForTest()
	})
	conf.Path = filepath.Join(t.TempDir(), "server.yaml")
	config := []byte(`
connsvr:
  debug:
    log_dir: logs
    log_level: info
  identity:
    self_bus_id: "1.1"
base_cfg:
  runtime:
    bus_mq_addr: "amqp://localhost"
    admin_server:
      enabled: true
      port: 0
    tracing:
      sampler_ratio: 0.25
`)
	if err := os.WriteFile(conf.Path, config, 0600); err != nil {
		t.Fatal(err)
	}
	assembly := ForGoOne()
	// This incomplete fixture must be rejected by the real service validators.
	if err := assembly.LoadServiceConfig("connsvr"); err == nil {
		t.Fatal("expected connsvr validation to reject incomplete configuration")
	}
	common := assembly.CommonForService("connsvr")
	if common.LogDir != "logs" || common.LogLevel != "info" || common.SelfBusId != "1.1" ||
		common.BusMQAddr != "amqp://localhost" || !common.AdminEnabled || common.AdminPort != 8101 ||
		common.Tracing.SamplerRatio != 0.25 {
		t.Fatalf("unexpected service configuration: %+v", common)
	}
	rules := assembly.RouteRules()
	if len(rules) == 0 || len(rules) != len(misc.ServerRouteRules) {
		t.Fatalf("unexpected route-rule count: %d", len(rules))
	}
	for route, want := range misc.ServerRouteRules {
		if got, ok := rules[route]; !ok || got != want {
			t.Fatalf("route %d: got %d, want %d", route, got, want)
		}
	}
}
