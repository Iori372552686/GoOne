package factory

import "testing"

// TestNewPublisher_Nacos 构造 nacos Publisher（客户端惰性连接，无需真实服务器）。
// 验证：URL 解析 + 后端分支正确，返回非 nil Publisher。
func TestNewPublisher_Nacos(t *testing.T) {
	cfg, err := ParseConfig("nacos://127.0.0.1:8848?dataid=ItemConfig.json&group=GOONE&namespace_id=public")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	pub, err := NewPublisher(cfg)
	if err != nil {
		t.Fatalf("NewPublisher: %v", err)
	}
	defer func() { _ = pub.Close() }()
	if cfg.Backend != BackendNacos {
		t.Fatalf("backend=%q want nacos", cfg.Backend)
	}
	if pub == nil {
		t.Fatal("publisher is nil")
	}
}

// TestNewPublisherFromURL_Nacos 走便捷入口，断言 cfg 与 publisher 同时返回。
func TestNewPublisherFromURL_Nacos(t *testing.T) {
	pub, cfg, err := NewPublisherFromURL("nacos://127.0.0.1:8848?dataid=a.json&group=G")
	if err != nil {
		t.Fatalf("NewPublisherFromURL: %v", err)
	}
	defer func() { _ = pub.Close() }()
	if cfg.NacosGroup != "G" {
		t.Fatalf("group=%q want G", cfg.NacosGroup)
	}
}

// TestNewPublisher_Etcd_Disabled 在默认 tag（未启用 config_etcd）下，
// etcd Publisher 必须返回 "not enabled" 错误，与读路径 newEtcdClient 行为一致。
func TestNewPublisher_Etcd_Disabled(t *testing.T) {
	cfg, err := ParseConfig("etcd://127.0.0.1:2379?path=/goone/config")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	pub, err := NewPublisher(cfg)
	if err == nil {
		_ = pub.Close()
		t.Fatal("want 'not enabled' error under default tags, got nil")
	}
}

// TestNewPublisher_UnsupportedBackend apollo/consul/k8s 暂未实现写路径。
func TestNewPublisher_UnsupportedBackend(t *testing.T) {
	cfg, err := ParseConfig("apollo://?appid=x&endpoint=http://127.0.0.1:8080&namespace=app.yaml")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if _, err := NewPublisher(cfg); err == nil {
		t.Fatal("want error for unsupported publisher backend")
	}
}

// TestParseConfig_EtcdAuth 断言新增的 etcd username/password query 解析。
func TestParseConfig_EtcdAuth(t *testing.T) {
	cfg, err := ParseConfig("etcd://127.0.0.1:2379?path=/goone/config&username=root&password=secret")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.EtcdUserName != "root" {
		t.Fatalf("username=%q want root", cfg.EtcdUserName)
	}
	if cfg.EtcdPassword != "secret" {
		t.Fatalf("password=%q want secret", cfg.EtcdPassword)
	}
	if cfg.Path != "/goone/config" {
		t.Fatalf("path=%q", cfg.Path)
	}
}
