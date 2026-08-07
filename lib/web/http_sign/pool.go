package http_sign

import "sync"

// builderPool 在签名计算之间复用 *poolBuffer，降低每请求热路径的 GC 压力。
//
// 归还（Put）时 buffer 会被重置为空（但保留底层数组），调用方在归还后
// 不得再持有或使用该 buffer。
var builderPool = sync.Pool{
	New: func() interface{} { return newPoolBuffer() },
}

// poolBuffer 封装 []byte，使池返回具体指针类型（比每次 Get 装箱
// interface{} 更省）；调用方可在归还前读取其字节内容。
type poolBuffer struct {
	buf []byte
}

func newPoolBuffer() *poolBuffer {
	return &poolBuffer{buf: make([]byte, 0, 256)}
}

// getBuffer 从池中取一个长度为 0 的 buffer。
func getBuffer() *poolBuffer {
	b := builderPool.Get().(*poolBuffer)
	b.buf = b.buf[:0]
	return b
}

// putBuffer 将 buffer 归还到池中。
//
// 过大的 buffer 不予回收，避免池中残留超长缓冲。
func putBuffer(b *poolBuffer) {
	if cap(b.buf) > 64*1024 {
		return
	}
	builderPool.Put(b)
}

func (b *poolBuffer) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *poolBuffer) WriteString(s string) (int, error) {
	b.buf = append(b.buf, s...)
	return len(s), nil
}

func (b *poolBuffer) Bytes() []byte { return b.buf }
