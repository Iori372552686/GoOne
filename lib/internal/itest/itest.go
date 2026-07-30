// Package itest 提供集成测试的统一门控与预检工具。
//
// 所有需要真实中间件（etcd、redis、mysql、rabbitmq、nacos、zookeeper、consul 等）
// 的测试统一通过 GOONE_INTEGRATION=1 启用，并在测试开始前做一次 TCP 预检：
//
//	func TestXxx(t *testing.T) {
//	    itest.Require(t, "127.0.0.1:2379") // 未开启集成模式或中间件不可达则 t.Skip
//	    ...
//	}
//
// 这样默认 `go test ./...` 在无中间件的开发机上会快速 Skip 而非超时失败，
// 仅在显式 `GOONE_INTEGRATION=1 go test ...` 时才执行真实集成用例。
package itest

import (
	"net"
	"os"
	"testing"
	"time"
)

// probeTimeout 是中间件可达性预检的超时，符合 V3-P0-02 计划的 500ms~1s 要求。
const probeTimeout = time.Second

// Enabled 报告是否启用了集成测试（GOONE_INTEGRATION=1）。
func Enabled() bool {
	return os.Getenv("GOONE_INTEGRATION") == "1"
}

// Require 是中间件集成测试的统一门控：
//  1. 未设置 GOONE_INTEGRATION=1 时 t.Skip（默认开发机快速跳过）。
//  2. 设置后对 addr 做 TCP 预检（probeTimeout），不可达则 t.Skip。
//
// addr 为 "host:port" 形式。调用方应在连接中间件之前调用本函数。
func Require(t *testing.T, addr string) {
	t.Helper()
	if !Enabled() {
		t.Skip("set GOONE_INTEGRATION=1 to run integration tests")
	}
	conn, err := net.DialTimeout("tcp", addr, probeTimeout)
	if err != nil {
		t.Skipf("integration dependency unavailable at %s: %v", addr, err)
	}
	_ = conn.Close()
}
