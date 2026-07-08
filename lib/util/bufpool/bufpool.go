// Package bufpool provides a shared []byte pool for per-message scratch
// buffers on hot paths (gateway write merging, frame assembly).
package bufpool

import "sync"

// maxPooledSize caps the buffers kept in the pool so a burst of large
// messages cannot pin large amounts of memory.
const maxPooledSize = 64 * 1024

var pool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 2048)
		return &b
	},
}

// Get returns a length-n buffer, reusing pooled memory when possible.
func Get(n int) []byte {
	bp := pool.Get().(*[]byte)
	if cap(*bp) < n {
		pool.Put(bp)
		return make([]byte, n)
	}
	return (*bp)[:n]
}

// Put recycles a buffer obtained from Get. The caller must not use the
// buffer afterwards.
func Put(b []byte) {
	if cap(b) == 0 || cap(b) > maxPooledSize {
		return
	}
	b = b[:0]
	pool.Put(&b)
}
