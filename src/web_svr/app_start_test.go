package websvr

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/Iori372552686/GoOne/lib/db/redis"
	"github.com/Iori372552686/GoOne/module/conf"
	"github.com/Iori372552686/GoOne/src/web_svr/globals"
	goredis "github.com/redis/go-redis/v9"
)

// closeCountingClient 统计 Close 调用次数的 Redis 桩（F08 回滚观测）。
type closeCountingClient struct {
	goredis.UniversalClient
	closeCalls int
}

func (c *closeCountingClient) Close() error {
	c.closeCalls++
	return nil
}

// F08 验收：Redis 起来后任一后续初始化步骤失败，Start 必须自行回收已启资源
//（App 契约不会为 Start 失败的组件调用 Stop）。此处用"缺 http_sign 配置"注入
// 第二步失败。
func TestStartRollsBackRedisOnLaterFailure(t *testing.T) {
	conf.ResetForTest()
	// db_instances 为空列表（OnStart 静默跳过、保留桩实例），但缺 http_sign 键
	// → 第二步 Unmarshal 失败 → 触发回滚。
	if err := conf.LoadBytes([]byte("base_cfg:\n  dependencies:\n    db_instances: []\n"), ".yaml"); err != nil {
		t.Fatalf("conf.LoadBytes: %v", err)
	}
	defer conf.ResetForTest()

	stub := &closeCountingClient{}
	oldMgr := globals.RedisMgr
	globals.RedisMgr = redis.NewRedisMgr()
	globals.RedisMgr.AddClientInstance(1, stub)
	defer func() { globals.RedisMgr = oldMgr }()

	w := &webRuntimeComponent{}
	if err := w.Start(context.Background()); err == nil {
		t.Fatal("缺少 http_sign 配置时 Start 应失败")
	}
	if stub.closeCalls != 1 {
		t.Fatalf("Start 失败应回滚关闭 Redis 池，实际 Close %d 次", stub.closeCalls)
	}
	if got := globals.RedisMgr.InstanceCount(); got != 0 {
		t.Fatalf("回滚后实例表应为空，实际 %d", got)
	}
}

// 成功路径不受回滚逻辑影响（配置完整 + 无 Redis 实例时 OnStart 静默跳过；
// HTTP 起在随机端口），由 t.Cleanup 走 Drain/Stop 归还端口。
func TestStartSucceedsWithFullConf(t *testing.T) {
	conf.ResetForTest()
	// 探测一个空闲端口供 StartGin 使用（其不接受 port 0）。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	freePort := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	yaml := fmt.Sprintf(`
base_cfg:
  dependencies:
    db_instances: []
    http_sign:
      - index_name: default
        private_key: k
        sign_name: sign
        expired_time: 1800
        timestamp_name: timestamp
        sign_type: md5
    rest_api_config: []
websvr:
  runtime:
    http_server:
      port: %d
      mode: test
    grpc_server:
      enabled: false
`, freePort)
	if err := conf.LoadBytes([]byte(yaml), ".yaml"); err != nil {
		t.Fatalf("conf.LoadBytes: %v", err)
	}
	defer conf.ResetForTest()

	w := &webRuntimeComponent{}
	if err := w.Start(context.Background()); err != nil {
		t.Fatalf("完整配置下 Start 应成功: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		_ = w.Drain(ctx)
		_ = w.Stop(ctx)
	})
	if w.getHTTPServer() == nil {
		t.Fatal("成功路径应记录 HTTP server")
	}
}
