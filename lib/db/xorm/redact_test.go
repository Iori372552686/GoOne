package orm

import (
	"strings"
	"testing"
)

// TestRedactDSNRemovesPassword 验证 DSN 脱敏后不含明文密码（敏感信息治理）。
func TestRedactDSNRemovesPassword(t *testing.T) {
	const secret = "p@ss-w0rd-2026"
	cases := []struct {
		name string
		dsn  string
	}{
		{"master", "game:" + secret + "@tcp(10.0.0.3:3306)/gamedb?timeout=3s&parseTime=true"},
		{"slave", "reader:" + secret + "@tcp(10.0.0.4:3306)/gamedb?charset=utf8"},
		{"no-params", "u:" + secret + "@tcp(127.0.0.1:3306)/db"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redactDSN(c.dsn)
			if strings.Contains(got, secret) {
				t.Fatalf("redactDSN leaks password: %q", got)
			}
			// 保留可诊断的主机/库/参数部分。
			at := strings.LastIndex(c.dsn, "@")
			if at >= 0 && !strings.HasSuffix(got, c.dsn[at:]) {
				t.Fatalf("redactDSN dropped suffix: got %q want suffix %q", got, c.dsn[at:])
			}
		})
	}
}

// TestRedactDSNNilAt 验证无 @ 的异常输入不 panic、原样返回。
func TestRedactDSNNoAt(t *testing.T) {
	in := "malformed-dsn"
	if got := redactDSN(in); got != in {
		t.Fatalf("expected passthrough %q, got %q", in, got)
	}
}

// TestRedactDSNsBatchPreservesOriginal 验证批量脱敏不改原切片。
func TestRedactDSNsBatchPreservesOriginal(t *testing.T) {
	const secret = "leak-me"
	src := []string{"u:" + secret + "@tcp(h:1)/db", "u2:" + secret + "@tcp(h2:2)/db2"}
	out := redactDSNs(src)
	if strings.Contains(src[0], secret) == false {
		t.Fatal("original must be unchanged by redactDSNs")
	}
	for i, o := range out {
		if strings.Contains(o, secret) {
			t.Fatalf("out[%d] leaks: %q", i, o)
		}
	}
}
