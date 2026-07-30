package redis

import (
	"strings"
	"testing"
)

// TestConfigSafeStringRedactsPassword 验证 Redis 配置的可日志表示不含密码，
// 仅保留实例 ID、地址、DB 与集群标记（V3-P0-01 敏感信息治理）。
func TestConfigSafeStringRedactsPassword(t *testing.T) {
	const secret = "super-secret-pass-123"
	c := Config{
		InstanceID: 7,
		IP:         "10.0.0.5",
		Port:       6379,
		Password:   secret,
		IsCluster:  true,
		DbIndex:    2,
		Description: "game-cache",
	}

	got := c.SafeString()

	if strings.Contains(got, secret) {
		t.Fatalf("SafeString leaks password: %q", got)
	}
	// 计划要求：只记录实例 ID、地址、DB 和连接池大小。
	for _, want := range []string{"instance:7", "10.0.0.5:6379", "db:2", "cluster:true"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SafeString missing %q in %q", want, got)
		}
	}
}

// TestConfigSafeStringEmptyPassword 验证空密码场景不 panic 且格式正常。
func TestConfigSafeStringEmptyPassword(t *testing.T) {
	c := Config{InstanceID: 1, IP: "127.0.0.1", Port: 6379, DbIndex: 0}
	got := c.SafeString()
	if !strings.Contains(got, "127.0.0.1:6379") {
		t.Fatalf("SafeString missing addr: %q", got)
	}
}
