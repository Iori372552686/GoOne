package bufpool

import "testing"

func TestGetPut(t *testing.T) {
	b := Get(100)
	if len(b) != 100 {
		t.Fatalf("expected len 100, got %d", len(b))
	}
	Put(b)

	big := Get(maxPooledSize + 1)
	if len(big) != maxPooledSize+1 {
		t.Fatalf("expected len %d, got %d", maxPooledSize+1, len(big))
	}
	Put(big) // oversized: silently dropped

	Put(nil) // must not panic
}

func TestAcquireRelease(t *testing.T) {
	bp := Acquire(100)
	if len(bp.Bytes) != 100 {
		t.Fatalf("expected len 100, got %d", len(bp.Bytes))
	}
	bp.Bytes[0] = 0xAB
	Release(bp)

	// 超大缓冲不进池。
	big := Acquire(maxPooledSize + 1)
	if len(big.Bytes) != maxPooledSize+1 {
		t.Fatalf("expected len %d, got %d", maxPooledSize+1, len(big.Bytes))
	}
	Release(big)

	Release(nil) // 必须 not panic
}

func BenchmarkGetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := Get(256)
		Put(buf)
	}
}

// BenchmarkAcquireRelease 度量 Lease 池化的稳态开销。目标：0 alloc。
// 与 BenchmarkGetPut 对照：Get/Put 因丢失 *Buffer 关联仍有 1 alloc，Acquire/Release
// 应为 0。
func BenchmarkAcquireRelease(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		bp := Acquire(256)
		Release(bp)
	}
}

