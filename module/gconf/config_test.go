package gconf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "server_conf.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadMainConfigSupportsGroupedFields(t *testing.T) {
	path := writeTempConfig(t, `
base_cfg:
  runtime:
    register_addr: "zk://127.0.0.1:2181?service=goone"
    bus_mq_addr: "amqp://guest:guest@127.0.0.1:5672/"
    admin_server:
      enabled: true
      ip: "127.0.0.1"
      port: 8111
  dependencies:
    db_instances:
      - instance_id: 1
        ip: "127.0.0.1"
        port: 6379
  debug:
    pprof: true
mainsvr:
  identity:
    self_bus_id: "1.1.2.1"
  debug:
    log_dir: "./logs"
    log_level: "info"
  capacity:
    trans_shard_count: 8
    role_sync_patch_enabled: true
    role_sync_patch_allow_uids: [1001, 1002]
    role_persist_debounce_sec: 15
`)

	if err := LoadMainConfig(path); err != nil {
		t.Fatalf("LoadMainConfig() error = %v", err)
	}

	if MainSvrCfg.CommonRuntime.RegisterAddr != "zk://127.0.0.1:2181?service=goone" {
		t.Fatalf("unexpected grouped register_addr: %q", MainSvrCfg.CommonRuntime.RegisterAddr)
	}
	if MainSvrCfg.Debug.LogLevel != "info" {
		t.Fatalf("debug log level = %q, want info", MainSvrCfg.Debug.LogLevel)
	}
	if MainSvrCfg.Capacity.TransShardCount != 8 {
		t.Fatalf("capacity trans_shard_count = %d, want 8", MainSvrCfg.Capacity.TransShardCount)
	}
	if !MainSvrCfg.Capacity.RoleSyncPatchEnabled {
		t.Fatalf("role_sync_patch_enabled should be true")
	}
	if MainSvrCfg.Capacity.RolePersistDebounceSec != 15 {
		t.Fatalf("role_persist_debounce_sec = %d, want 15", MainSvrCfg.Capacity.RolePersistDebounceSec)
	}
	if !MainSvrCfg.CommonDebug.Pprof {
		t.Fatalf("pprof flag should be true")
	}
	if got := len(MainSvrCfg.Dependencies.DbInstances); got != 1 {
		t.Fatalf("expected 1 db instance, got %d", got)
	}
}

func TestLoadConnConfigSupportsGroupedFields(t *testing.T) {
	path := writeTempConfig(t, `
base_cfg:
  runtime:
    register_addr: "zk://127.0.0.1:2181?service=goone"
    bus_mq_addr: "amqp://guest:guest@127.0.0.1:5672/"
  debug:
    pprof: true
connsvr:
  identity:
    self_bus_id: "1.1.1.1"
  debug:
    log_dir: "./logs"
    log_level: "debug"
  runtime:
    listen_port: 11000
`)

	if err := LoadConnConfig(path); err != nil {
		t.Fatalf("LoadConnConfig() error = %v", err)
	}

	if ConnSvrCfg.CommonRuntime.BusMQAddr != "amqp://guest:guest@127.0.0.1:5672/" {
		t.Fatalf("unexpected grouped bus_mq_addr: %q", ConnSvrCfg.CommonRuntime.BusMQAddr)
	}
	if ConnSvrCfg.Identity.SelfBusId != "1.1.1.1" {
		t.Fatalf("self_bus_id = %q, want 1.1.1.1", ConnSvrCfg.Identity.SelfBusId)
	}
	if ConnSvrCfg.Runtime.ListenPort != 11000 {
		t.Fatalf("listen_port = %d, want 11000", ConnSvrCfg.Runtime.ListenPort)
	}
	if ConnSvrCfg.Debug.LogDir != "./logs" {
		t.Fatalf("log_dir = %q, want ./logs", ConnSvrCfg.Debug.LogDir)
	}
	if !ConnSvrCfg.CommonDebug.Pprof {
		t.Fatalf("pprof flag should be true")
	}
}

func TestLoadWebConfigFailsFastOnInvalidRuntime(t *testing.T) {
	path := writeTempConfig(t, `
base_cfg:
  dependencies:
    db_instances:
      - instance_id: 1
        ip: "127.0.0.1"
        port: 6379
websvr:
  identity:
    self_bus_id: "1.1.12.1"
  debug:
    log_dir: "./logs"
    log_level: "info"
  runtime:
    http_server:
      ip: ""
      port: 0
      session_name: "GoOne@Web"
      mode: "debug"
      auth_enable: false
`)

	err := LoadWebConfig(path)
	if err == nil {
		t.Fatalf("expected LoadWebConfig() to fail")
	}
	if !strings.Contains(err.Error(), "websvr.runtime.http_server.port") {
		t.Fatalf("unexpected error: %v", err)
	}
}
