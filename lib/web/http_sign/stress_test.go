package http_sign

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 本文件聚焦性能压测与并发安全验证：
//   - 并发 PushSign/CheckSign（配合 -race 检测数据竞争）
//   - 不同参数规模下的 Map2uri / buildSign / CheckSign 吞吐
//   - Sync.Pool 复用有效性（对比无 pool 的等价实现）
//   - 完整 PushSign→CheckSign 往返开销

// makeParams 生成指定规模的有序参数 map。
func makeParams(n int) map[string]string {
	m := make(map[string]string, n)
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("key_%04d", i)] = strconv.Itoa(i)
	}
	return m
}

// ---------------- 并发安全压测（务必配合 -race 运行）----------------

// BenchmarkCheckSign_Parallel 并发校验签名，验证 HttpSign 在多 goroutine 下
// 的正确性与吞吐。运行：go test -race -bench=BenchmarkCheckSign_Parallel
func BenchmarkCheckSign_Parallel(b *testing.B) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix(), 10)}
	body := []byte(`{"hello":"world"}`)
	s.PushSign(params, body)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if code, _ := s.CheckSign(params, body, ""); code != SignOK {
				b.Fatalf("并发校验应通过: code=%s", code)
			}
		}
	})
}

// BenchmarkPushSign_Parallel 并发签名，验证 PushSign 内部（含 uuid 生成、
// map 写入）的并发安全。
func BenchmarkPushSign_Parallel(b *testing.B) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	body := []byte(`{"hello":"world"}`)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 每次新建 map，避免并发写同一 map 触发 race
			p := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix(), 10)}
			s.PushSign(p, body)
		}
	})
}

// BenchmarkSignMgr_ParallelGet 并发 GetSignIns，验证 RWMutex 读路径无竞争。
func BenchmarkSignMgr_ParallelGet(b *testing.B) {
	m := NewSignMgr()
	m.SetSignIns("default", BuildHttpSign("sign", "secret", 0, "timestamp", "", "1"))
	m.SetSignIns("other", BuildHttpSign("sign", "secret2", 0, "timestamp", "", "1"))

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				_ = m.GetSignIns()
			} else {
				_ = m.GetSignIns("other")
			}
			i++
		}
	})
}

// TestStress_ConcurrentSafetyNoRace 高强度并发混合读写 HttpSign 与 SignMgr，
// 配合 -race 检测，确保无数据竞争且校验结果正确。
func TestStress_ConcurrentSafetyNoRace(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "request_id", "1").WithSignType(Sign_HMacSha256)
	mgr := NewSignMgr()
	mgr.SetSignIns("default", s)

	const goroutines = 16
	const iterations = 500
	var wg sync.WaitGroup
	var fail int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				ins := mgr.GetSignIns() // 并发读
				params := map[string]string{
					"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
				}
				body := []byte(fmt.Sprintf(`{"g":%d,"i":%d}`, id, i))
				ins.PushSign(params, body) // 并发写各自 map

				if code, _ := ins.CheckSign(params, body, ""); code != SignOK {
					atomic.AddInt64(&fail, 1)
				}

				// 篡改后应失败
				if code, _ := ins.CheckSign(params, []byte("tampered"), ""); code != ErrVerifyFailure {
					atomic.AddInt64(&fail, 1)
				}
			}
		}(g)
	}
	wg.Wait()

	if fail != 0 {
		t.Fatalf("并发校验出现 %d 次非预期结果", fail)
	}
}

// ---------------- 规模扩展压测 ----------------

// 各规模参数下 Map2uri 的吞吐，验证 strings.Builder + 预分配在不同规模的表现。
func BenchmarkMap2uri_Scale(b *testing.B) {
	for _, n := range []int{1, 5, 10, 50, 100} {
		b.Run(fmt.Sprintf("params=%d", n), func(b *testing.B) {
			params := makeParams(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = Map2uri(params, "", true, false)
			}
		})
	}
}

// 各规模参数下 buildSign 的吞吐。
func BenchmarkBuildSign_Scale(b *testing.B) {
	for _, n := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("md5/params=%d", n), func(b *testing.B) {
			s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
			params := makeParams(n)
			body := []byte(`{"hello":"world"}`)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = s.buildSign(params, body, Sign_Md5, Version_NewV1)
			}
		})
	}
}

// 三种算法在同一参数规模下的对比。
func BenchmarkBuildSign_AlgoCompare(b *testing.B) {
	params := makeParams(20)
	body := []byte(`{"hello":"world"}`)
	for _, tc := range []struct {
		name string
		algo ESignType
	}{
		{"md5", Sign_Md5},
		{"sha1", Sign_Sha1},
		{"hmac_sha256", Sign_HMacSha256},
	} {
		s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = s.buildSign(params, body, tc.algo, Version_NewV1)
			}
		})
	}
}

// ---------------- 完整往返压测 ----------------

// BenchmarkPushSignThenCheckSign 度量“签名+校验”一次完整往返的开销，
// 这是真实请求链路（如 verifier）的每请求成本。
func BenchmarkPushSignThenCheckSign(b *testing.B) {
	for _, algo := range []ESignType{Sign_Md5, Sign_HMacSha256} {
		b.Run(string(algo), func(b *testing.B) {
			s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1").WithSignType(algo)
			body := []byte(`{"hello":"world"}`)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p := map[string]string{}
				s.PushSign(p, body)
				_, _ = s.CheckSign(p, body, "")
			}
		})
	}
}

// BenchmarkUriParam2Map 度量查询串解析性能（verifier 入口路径）。
func BenchmarkUriParam2Map(b *testing.B) {
	// 模拟带签名的真实查询串
	raw := "timestamp=1700000000&user=alice&action=login&sign=484ea76937e5a7e1bddf7dec4362cd22&request_id=abc123"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = UriParam2Map(raw)
	}
}

// BenchmarkPool_Reuse 验证 sync.Pool 在高并发签名计算下的复用效果：
// 多轮 buildSign 应主要命中池而非新建 buffer。
func BenchmarkPool_Reuse(b *testing.B) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
	params := makeParams(10)
	body := []byte(`{"hello":"world"}`)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.buildSign(params, body, Sign_Md5, Version_NewV1)
		}
	})
}

// TestStress_HighVolumeSignVerify 大批量签名+校验的正确性回归，
// 确保 Pool 复用不会造成跨请求数据污染（如残留旧 buffer 内容）。
func TestStress_HighVolumeSignVerify(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1").WithSignType(Sign_HMacSha256)
	const n = 5000
	for i := 0; i < n; i++ {
		params := map[string]string{
			"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
			"seq":       strconv.Itoa(i),
		}
		body := []byte(fmt.Sprintf(`{"i":%d}`, i))
		s.PushSign(params, body)
		if code, _ := s.CheckSign(params, body, ""); code != SignOK {
			t.Fatalf("第 %d 次校验失败（疑似 Pool 数据污染）: code=%s", i, code)
		}
	}
}
