package conf

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// 各格式等价的最小配置：覆盖标量、嵌套 map、数组、数组下标、不同类型。
const yamlSrc = `
base_cfg:
  runtime:
    register_addr: 'etcd://127.0.0.1:2379'
    bus_mq_addr: 'amqp://x@y/'
    admin_server: {enabled: true, ip: "", port: 0}
    tracing: {enabled: false, sampler_ratio: 0.5}
  dependencies:
    db_instances:
      - {instance_id: 1, ip: "10.0.0.1", port: 6379}
      - {instance_id: 3, ip: "10.0.0.2", port: 6380}
    orm_instances:
      - {index_name: default, master: {ip: "10.0.0.3", port: 3306}}
connsvr:
  identity: {self_bus_id: "2.1.1.1"}
  runtime: {listen_port: 11000, tcp_impl_type: gonet, kcp_port: 0}
  capacity: {max_connections: 1000, overload_mode: enforce}
mainsvr:
  identity: {self_bus_id: "2.1.2.1"}
  capacity: {trans_shard_count: 4, role_persist_debounce_sec: 5}
websvr:
  identity: {self_bus_id: "2.1.12.1"}
  runtime:
    http_server: {port: 10001}
    grpc_server: {enabled: false}
`

const tomlSrc = `
[base_cfg.runtime]
register_addr = "etcd://127.0.0.1:2379"
bus_mq_addr = "amqp://x@y/"

[base_cfg.runtime.admin_server]
enabled = true
ip = ""
port = 0

[base_cfg.runtime.tracing]
enabled = false
sampler_ratio = 0.5

[[base_cfg.dependencies.db_instances]]
instance_id = 1
ip = "10.0.0.1"
port = 6379
[[base_cfg.dependencies.db_instances]]
instance_id = 3
ip = "10.0.0.2"
port = 6380

[[base_cfg.dependencies.orm_instances]]
index_name = "default"
[base_cfg.dependencies.orm_instances.master]
ip = "10.0.0.3"
port = 3306

[connsvr.identity]
self_bus_id = "2.1.1.1"
[connsvr.runtime]
listen_port = 11000
tcp_impl_type = "gonet"
kcp_port = 0
[connsvr.capacity]
max_connections = 1000
overload_mode = "enforce"
[mainsvr.identity]
self_bus_id = "2.1.2.1"
[mainsvr.capacity]
trans_shard_count = 4
role_persist_debounce_sec = 5
[websvr.identity]
self_bus_id = "2.1.12.1"
[websvr.runtime.http_server]
port = 10001
[websvr.runtime.grpc_server]
enabled = false
`

const jsonSrc = `{
  "base_cfg": {
    "runtime": {
      "register_addr": "etcd://127.0.0.1:2379",
      "bus_mq_addr": "amqp://x@y/",
      "admin_server": {"enabled": true, "ip": "", "port": 0},
      "tracing": {"enabled": false, "sampler_ratio": 0.5}
    },
    "dependencies": {
      "db_instances": [
        {"instance_id": 1, "ip": "10.0.0.1", "port": 6379},
        {"instance_id": 3, "ip": "10.0.0.2", "port": 6380}
      ],
      "orm_instances": [
        {"index_name": "default", "master": {"ip": "10.0.0.3", "port": 3306}}
      ]
    }
  },
  "connsvr": {
    "identity": {"self_bus_id": "2.1.1.1"},
    "runtime": {"listen_port": 11000, "tcp_impl_type": "gonet", "kcp_port": 0},
    "capacity": {"max_connections": 1000, "overload_mode": "enforce"}
  },
  "mainsvr": {
    "identity": {"self_bus_id": "2.1.2.1"},
    "capacity": {"trans_shard_count": 4, "role_persist_debounce_sec": 5}
  },
  "websvr": {
    "identity": {"self_bus_id": "2.1.12.1"},
    "runtime": {"http_server": {"port": 10001}, "grpc_server": {"enabled": false}}
  }
}`

// loadSrc 按格式加载，reset 保证测试间互不干扰（Load 幂等）。
func loadSrc(t *testing.T, src, ext string) {
	t.Helper()
	reset()
	if err := LoadBytes([]byte(src), ext); err != nil {
		t.Fatalf("LoadBytes(%s): %v", ext, err)
	}
}

func TestThreeFormatsEquivalent(t *testing.T) {
	for _, c := range []struct{ name, src, ext string }{
		{"yaml", yamlSrc, ".yaml"},
		{"yml", yamlSrc, ".yml"},
		{"toml", tomlSrc, ".toml"},
		{"json", jsonSrc, ".json"},
	} {
		t.Run(c.name, func(t *testing.T) {
			loadSrc(t, c.src, c.ext)
			// 标量
			if got := Get("connsvr.runtime.listen_port").Int(); got != 11000 {
				t.Fatalf("listen_port = %d, want 11000", got)
			}
			if got := Get("connsvr.identity.self_bus_id").String(); got != "2.1.1.1" {
				t.Fatalf("self_bus_id = %q", got)
			}
			// 数组下标
			if got := Get("base_cfg.dependencies.db_instances.0.ip").String(); got != "10.0.0.1" {
				t.Fatalf("db.0.ip = %q", got)
			}
			if got := Get("base_cfg.dependencies.db_instances.1.port").Int(); got != 6380 {
				t.Fatalf("db.1.port = %d", got)
			}
			// 数组对象内嵌套（orm.master.ip）—— TOML 的痛点场景，验证统一为 map
			if got := Get("base_cfg.dependencies.orm_instances.0.master.ip").String(); got != "10.0.0.3" {
				t.Fatalf("orm.master.ip = %q", got)
			}
			// bool + float
			if !Get("base_cfg.runtime.admin_server.enabled").Bool() {
				t.Fatal("admin enabled = false")
			}
			if got := Get("base_cfg.runtime.tracing.sampler_ratio").Float64(); got != 0.5 {
				t.Fatalf("sampler_ratio = %v", got)
			}
		})
	}
}

func TestMissingKeyReturnsDefault(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	if v := Get("does.not.exist", "fallback").String(); v != "fallback" {
		t.Fatalf("default fallback = %q", v)
	}
	if v := Get("does.not.exist"); v.Exists() {
		t.Fatal("missing key should not exist")
	}
	if got := Get("does.not.exist").Int(); got != 0 {
		t.Fatalf("missing int = %d", got)
	}
}

func TestHasAndExists(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	if !Has("connsvr.runtime.listen_port") {
		t.Fatal("Has(listen_port) = false")
	}
	if Has("connsvr.runtime.no_such_field") {
		t.Fatal("Has(no_such_field) = true")
	}
}

func TestLoadIdempotent(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	if err := LoadBytes([]byte(yamlSrc), ".yaml"); !errors.Is(err, ErrAlreadyLoaded) {
		t.Fatalf("二次 Load 应返回 ErrAlreadyLoaded，实际 %v", err)
	}
}

func TestGetBeforeLoad(t *testing.T) {
	reset()
	if v := Get("anything", "def"); v.String() != "def" {
		t.Fatalf("Load 前 Get 应返回默认值，实际 %q", v.String())
	}
	if err := Unmarshal("x", new(map[string]any)); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("Load 前 Unmarshal 应返回 ErrNotLoaded，实际 %v", err)
	}
}

func TestUnsupportedFormat(t *testing.T) {
	reset()
	if err := LoadBytes([]byte("x"), ".xml"); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("未知扩展名应返回 ErrUnsupportedFormat，实际 %v", err)
	}
}

func TestUnmarshalIntoStruct(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	type db struct {
		InstanceID int    `yaml:"instance_id"`
		IP         string `yaml:"ip"`
		Port       int    `yaml:"port"`
	}
	var dbs []db
	if err := Unmarshal("base_cfg.dependencies.db_instances", &dbs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(dbs) != 2 || dbs[0].IP != "10.0.0.1" || dbs[1].Port != 6380 {
		t.Fatalf("Unmarshal 结果不符: %+v", dbs)
	}
}

func TestValueTypedAccess(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	// Duration：字符串解析
	reset()
	if err := LoadBytes([]byte(`x: "5s"`), ".yaml"); err != nil {
		t.Fatal(err)
	}
	if d := Get("x").Duration(); d != 5*time.Second {
		t.Fatalf("Duration = %v", d)
	}
	// Ints / Strings 集合访问
	reset()
	if err := LoadBytes([]byte(`a: [1, 2, 3]
b: ["x", "y"]`), ".yaml"); err != nil {
		t.Fatal(err)
	}
	if got := Get("a").Ints(); len(got) != 3 || got[2] != 3 {
		t.Fatalf("Ints = %v", got)
	}
	if got := Get("b").Strings(); len(got) != 2 || got[1] != "y" {
		t.Fatalf("Strings = %v", got)
	}
}

func TestConcurrentReadNoLock(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Get("connsvr.runtime.listen_port").Int()
			_ = Get("base_cfg.dependencies.db_instances.0.ip").String()
			_ = Current()
		}()
	}
	wg.Wait()
}

func TestWatchNotSupported(t *testing.T) {
	_, err := Watch("x", nil)
	if !errors.Is(err, ErrWatchNotSupported) {
		t.Fatalf("Watch 应返回 ErrWatchNotSupported，实际 %v", err)
	}
}

// ---- 校验器 ----

func TestValidatorsPass(t *testing.T) {
	loadSrc(t, yamlSrc, ".yaml")
	for _, svc := range []string{"connsvr", "mainsvr", "websvr"} {
		if err := RunValidators(svc); err != nil {
			t.Fatalf("RunValidators(%s): %v", svc, err)
		}
	}
}

func TestValidatorsRejectBadConfig(t *testing.T) {
	// connsvr listen_port 缺失
	reset()
	bad := `
base_cfg:
  runtime:
    register_addr: 'etcd://x'
    bus_mq_addr: 'amqp://x'
connsvr:
  identity: {self_bus_id: "1"}
`
	if err := LoadBytes([]byte(bad), ".yaml"); err != nil {
		t.Fatal(err)
	}
	if err := RunValidators("connsvr"); err == nil {
		t.Fatal("缺 listen_port 应校验失败")
	}
	// mainsvr db_instances 空
	reset()
	bad2 := `
base_cfg: {runtime: {register_addr: 'etcd://x', bus_mq_addr: 'amqp://x'}}
mainsvr: {identity: {self_bus_id: "1"}}
`
	if err := LoadBytes([]byte(bad2), ".yaml"); err != nil {
		t.Fatal(err)
	}
	if err := RunValidators("mainsvr"); err == nil {
		t.Fatal("db_instances 空应校验失败")
	}
}

func TestResolveAdminPort(t *testing.T) {
	if p := ResolveAdminPort(0, "connsvr"); p != 8101 {
		t.Fatalf("connsvr 默认端口 = %d", p)
	}
	if p := ResolveAdminPort(0, "websvr"); p != 8112 {
		t.Fatalf("websvr 默认端口 = %d", p)
	}
	if p := ResolveAdminPort(9999, "connsvr"); p != 9999 {
		t.Fatalf("显式端口应原样返回 = %d", p)
	}
	if p := ResolveAdminPort(0, "unknown"); p != 0 {
		t.Fatalf("未知服务应保持 0 = %d", p)
	}
}
