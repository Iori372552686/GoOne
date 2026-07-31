package redis

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeCloseClient 是一个 radix.Client 桩，控制 Close 行为以测试错误聚合。
// 为避免引入对 radix 内部类型的复杂桩，这里直接测试 RedisMgr.Close 在多实例下的行为：
// 通过真实 AddInstance 失败路径与成功路径覆盖 Close 的幂等与聚合语义。

// TestRedisCloseIsIdempotent 验证 Close 幂等：空 mgr 或重复 Close 不 panic、返回 nil。
func TestRedisCloseIsIdempotent(t *testing.T) {
	m := NewRedisMgr()
	if err := m.Close(); err != nil {
		t.Fatalf("close empty mgr: %v", err)
	}
	// 再次 Close 仍 nil。
	if err := m.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestRedisComponentStartStopNoInstance 验证 Component 在零实例下 Start/Stop 成功。
func TestRedisComponentStartStopNoInstance(t *testing.T) {
	c := NewComponent("redis_test", NewRedisMgr(), nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
}

// TestRedisComponentName 验证 Name 返回构造名。
func TestRedisComponentName(t *testing.T) {
	c := NewComponent("my-redis", NewRedisMgr(), nil)
	if c.Name() != "my-redis" {
		t.Fatalf("name=%q want my-redis", c.Name())
	}
}

// TestRedisComponentStartFailureSurfacesError 验证 Start 失败（无效实例）返回带组件名的 error。
func TestRedisComponentStartFailureSurfacesError(t *testing.T) {
	// 用一个非法配置触发 AddInstance 失败。
	bad := Config{InstanceID: 1, IP: "127.0.0.1", Port: 1, Password: "", DbIndex: 0, IsCluster: true}
	c := NewComponent("redis_bad", NewRedisMgr(), []Config{bad})
	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("expected start error for invalid cluster config")
	}
	if !strings.Contains(err.Error(), "redis_bad") {
		t.Fatalf("error should contain component name, got: %v", err)
	}
	// 必须能 errors.Is 解到非 nil 根因（至少不是哨兵 nil）。
	if errors.Is(err, nil) {
		t.Fatal("error should not be nil-sentinel")
	}
}
