//go:build config_etcd
// +build config_etcd

package test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/contrib/config/factory"
	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/base"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/manager"
	"github.com/Iori372552686/GoOne/tools/cfgtool/internal/parser"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// defaultUploadAddr 是真实可用的 etcd 地址（由用户提供）。
// 可用环境变量 CFGTOOL_UPLOAD_ADDR 覆盖，例如带鉴权：
//   CFGTOOL_UPLOAD_ADDR='etcd://user:pwd@47.107.101.29:2379?path=/goone/cfgtool-test&username=user&password=pwd'
const defaultUploadAddr = "etcd://47.107.101.29:2379?path=/goone/cfgtool-test"

// TestUploadEtcdRoundTrip 端到端：
//   xls(已签入) → GenProto → ParseProto → GenData → UploadData → 读回校验。
//
// 该测试依赖外网 etcd，默认跳过；设置 CFGTOOL_UPLOAD_ADDR 或 CFGTOOL_UPLOAD_RUN=1 启用。
// 该测试仅在被 -tags config_etcd 编译时存在。
func TestUploadEtcdRoundTrip(t *testing.T) {
	addr := strings.TrimSpace(os.Getenv("CFGTOOL_UPLOAD_ADDR"))
	if addr == "" {
		// 默认地址已知，但仅当显式要求运行时才连外网，避免 CI 误触。
		if os.Getenv("CFGTOOL_UPLOAD_RUN") == "" {
			t.Skip("跳过 etcd 集成测试；设置 CFGTOOL_UPLOAD_ADDR 或 CFGTOOL_UPLOAD_RUN=1 启用")
		}
		addr = defaultUploadAddr
	}

	resetGlobals(t)
	root := repoRoot(t)

	// 1. 配置各产物输出到临时目录
	tmp := t.TempDir()
	domain.XlsxPath = filepath.Join(root, "tools", "cfgtool", "xls")
	domain.JsonPath = filepath.Join(tmp, "json")
	domain.TextPath = filepath.Join(tmp, "text")
	domain.BytesPath = filepath.Join(tmp, "bytes")
	domain.ProtoPath = filepath.Join(tmp, "proto")
	domain.Module = "github.com/Iori372552686/GoOne"
	domain.PbPath = "github.com/Iori372552686/game_protocol/protocol"
	domain.PkgName = filepath.Base(domain.PbPath)
	domain.ConfMode = "all"

	// 2. 上传参数：json + conf + bytes
	domain.UploadURL = addr
	domain.UploadType = "json,conf,bytes"

	// 3. 解析 etcd path（用于回读时构造 Source 路径 + 清理）
	cfg, err := factory.ParseConfig(addr)
	if err != nil {
		t.Fatalf("ParseConfig upload addr: %v", err)
	}
	if cfg.Backend != factory.BackendEtcd {
		t.Fatalf("本测试仅验证 etcd 后端，got backend=%q", cfg.Backend)
	}
	etcdPath := cfg.Path
	t.Logf("etcd path=%s addrs=%v", etcdPath, cfg.Addrs)

	// 清理残留（前缀删除），并在测试结束时再次清理。
	ec, derr := clientv3.New(clientv3.Config{Endpoints: cfg.Addrs, DialTimeout: 5 * time.Second,
		Username: cfg.EtcdUserName, Password: cfg.EtcdPassword})
	if derr != nil {
		t.Fatalf("dial etcd for cleanup: %v", derr)
	}
	cleanPrefix := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = ec.Delete(ctx, etcdPath, clientv3.WithPrefix())
	}
	cleanPrefix()
	defer func() {
		cleanPrefix()
		_ = ec.Close()
	}()

	// 4. 跑生成主流程
	files, err := base.Glob(domain.XlsxPath, ".*\\.xlsx", true)
	if err != nil {
		t.Fatalf("glob xlsx: %v", err)
	}
	if len(files) == 0 {
		t.Skipf("no xlsx fixtures under %s", domain.XlsxPath)
	}
	if err := parser.ParseFiles(files...); err != nil {
		t.Fatalf("ParseFiles: %v", err)
	}
	if err := service.GenProto(); err != nil {
		t.Fatalf("GenProto: %v", err)
	}
	if err := service.SaveProto(); err != nil {
		t.Fatalf("SaveProto: %v", err)
	}
	if err := manager.ParseProto(); err != nil {
		t.Fatalf("ParseProto: %v", err)
	}
	if err := service.GenData(); err != nil {
		t.Fatalf("GenData: %v", err)
	}
	t.Logf("生成完成，json=%s text=%s bytes=%s", domain.JsonPath, domain.TextPath, domain.BytesPath)

	// 5. 上传到 etcd
	if err := service.UploadData(); err != nil {
		t.Fatalf("UploadData: %v", err)
	}

	// 6. 回读：用 etcd 读 Source（prefix 模式）Load 全部 key
	cli, err := factory.NewClient(factory.Config{
		Backend:       factory.BackendEtcd,
		Addrs:         cfg.Addrs,
		Path:          etcdPath,
		EtcdPrefix:    true,
		EtcdUserName:  cfg.EtcdUserName,
		EtcdPassword:  cfg.EtcdPassword,
		Timeout:       10 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient(etcd read): %v", err)
	}
	defer func() { _ = cli.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = ctx // Source.Load 内部自管 ctx
	kvs, err := cli.Load()
	if err != nil {
		t.Fatalf("Load from etcd: %v", err)
	}
	t.Logf("回读到 %d 个 key", len(kvs))

	gotKeys := map[string]bool{}
	for _, kv := range kvs {
		gotKeys[filepath.Base(kv.Key)] = true
		if len(kv.Value) == 0 {
			t.Errorf("key %q 内容为空", kv.Key)
		}
	}

	// 7. 断言每个本地产物文件都有对应 etcd key（dataID = 文件名）
	for _, sub := range []string{"json", "text", "bytes"} {
		var dir string
		var ext string
		switch sub {
		case "json":
			dir, ext = domain.JsonPath, ".json"
		case "text":
			dir, ext = domain.TextPath, ".conf"
		case "bytes":
			dir, ext = domain.BytesPath, ".bytes"
		}
		fs, _ := base.Glob(dir, ".*\\"+ext+"$", false)
		for _, f := range fs {
			dataID := filepath.Base(f)
			if !gotKeys[dataID] {
				t.Errorf("etcd 缺少 dataID=%s（本地文件 %s 未上传成功）", dataID, f)
			}
		}
		if len(fs) == 0 {
			t.Errorf("本地 %s/*.%s 无产物文件", dir, ext)
		}
	}
}
