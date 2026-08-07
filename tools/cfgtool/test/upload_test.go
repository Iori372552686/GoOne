package test

import (
	"testing"

	"github.com/Iori372552686/GoOne/tools/cfgtool/domain"
	"github.com/Iori372552686/GoOne/tools/cfgtool/service"
)

// TestParseUploadType 验证 -uptype 字符串 → 上传项映射、去重、大小写归一与未知 token 跳过。
// 纯函数测试，无需网络、无需文件。
func TestParseUploadType(t *testing.T) {
	resetGlobals(t)
	// 给各目录填占位值，验证映射正确指向对应目录
	domain.JsonPath = "/tmp/json"
	domain.TextPath = "/tmp/text"
	domain.BytesPath = "/tmp/bytes"
	domain.LuaPath = "/tmp/lua"

	t.Run("empty", func(t *testing.T) {
		if got := service.ParseUploadType(""); got != nil {
			t.Fatalf("want nil for empty, got %v", got)
		}
		if got := service.ParseUploadType("   "); got != nil {
			t.Fatalf("want nil for blank, got %v", got)
		}
	})

	t.Run("all four", func(t *testing.T) {
		got := service.ParseUploadType("json,conf,bytes,lua")
		if len(got) != 4 {
			t.Fatalf("want 4 items, got %d: %+v", len(got), got)
		}
		want := []service.UploadItem{
			{Dir: "/tmp/json", Suffix: ".json", Format: "json"},
			{Dir: "/tmp/text", Suffix: ".conf", Format: "text"},
			{Dir: "/tmp/bytes", Suffix: ".bytes", Format: "bytes"},
			{Dir: "/tmp/lua", Suffix: ".lua", Format: "text"},
		}
		for i, w := range want {
			if got[i] != w {
				t.Errorf("item[%d]=%+v want %+v", i, got[i], w)
			}
		}
	})

	t.Run("dedup and order", func(t *testing.T) {
		// 重复与空白混合，应保留首次出现顺序
		got := service.ParseUploadType("json, json ,,bytes,JSON")
		if len(got) != 2 {
			t.Fatalf("want 2 items after dedup, got %d: %+v", len(got), got)
		}
		if got[0].Suffix != ".json" || got[1].Suffix != ".bytes" {
			t.Fatalf("order/suffix wrong: %+v", got)
		}
	})

	t.Run("unknown skipped", func(t *testing.T) {
		got := service.ParseUploadType("json,xml,lua")
		if len(got) != 2 {
			t.Fatalf("want 2 items (xml skipped), got %d: %+v", len(got), got)
		}
		if got[0].Suffix != ".json" || got[1].Suffix != ".lua" {
			t.Fatalf("unexpected: %+v", got)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		got := service.ParseUploadType("JSON")
		if len(got) != 1 || got[0].Suffix != ".json" {
			t.Fatalf("want json item, got %+v", got)
		}
	})
}

// TestUploadData_NoUploadURL verifies UploadData is a no-op when -upload is empty.
func TestUploadData_NoUploadURL(t *testing.T) {
	resetGlobals(t)
	domain.UploadURL = ""
	if err := service.UploadData(); err != nil {
		t.Fatalf("UploadData with empty URL should be no-op, got %v", err)
	}
}
