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

func BenchmarkGetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf := Get(256)
		Put(buf)
	}
}
