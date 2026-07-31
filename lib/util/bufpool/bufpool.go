// Package bufpool 提供共享的字节缓冲池，用于热路径（网关写合并、帧装配）的每消息
// 暂存缓冲。
//
// 池化模型：
//
//   - Acquire 返回一个 *Buffer（Lease），其 Bytes 字段为可用字节切片。
//   - Release 把 Buffer 归还池。调用方 Release 后不得再引用 Bytes。
//   - 池化的对象是 *Buffer 指针本身（固定大小，不随 payload 增长），故 sync.Pool 的
//     New 不产生逃逸分配；稳态 Get/Put 为 0 分配。
//
// Ownership 约定（写入队列与异步写必须遵守）：
//
//   - Acquire 后调用方拥有 Lease。
//   - 入队成功后 writer 拥有 Lease。
//   - 入队失败由 sender 释放。
//   - writer 完成、出错或连接关闭都必须只释放一次。
//   - Stop 必须释放队列残留。
//   - 超过 64 KiB 的 Buffer 不归还池（避免突发大消息钉住大量内存）。
package bufpool

import "sync"

// maxPooledSize 限制进入池的缓冲大小，使突发大消息无法钉住大量内存。
const maxPooledSize = 64 * 1024

// Buffer 是一个池化的字节 Lease。Acquire 返回 *Buffer，Release 归还。
// 调用方通过 Bytes 读写，Release 后不得再引用 Bytes。
type Buffer struct {
	Bytes []byte
}

var pool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 2048)
		return &Buffer{Bytes: b}
	},
}

// Acquire 返回一个 Bytes 长度为 size 的 Buffer（Lease）。优先复用池化内存；
// size 超出池化容量时返回一个新建的 Buffer（不进入池）。调用方必须在用完后调用
// Release 释放，且只释放一次。
func Acquire(size int) *Buffer {
	bp := pool.Get().(*Buffer)
	if cap(bp.Bytes) < size {
		// 池中的 Buffer 容量不足：新建一个临时 Buffer，不归还池。
		// 注意：不把旧的 *Buffer 扔回池（它容量小，可被 GC；保持池里只留够大的）。
		// 为避免丢失原 Buffer，这里扩容后仍用同一个 *Buffer。
		bp.Bytes = make([]byte, size)
		return bp
	}
	bp.Bytes = bp.Bytes[:size]
	return bp
}

// Release 归还一个 Acquire 得到的 Buffer。调用方此后不得再引用其 Bytes。
// 超过 maxPooledSize 的 Buffer 不归还池（直接丢弃，交由 GC）。Release 必须只调用
// 一次；重复 Release 是调用方 bug（会导致池对象被并发复用）。
func Release(bp *Buffer) {
	if bp == nil {
		return
	}
	if cap(bp.Bytes) > maxPooledSize {
		// 大缓冲不池化，避免钉住内存。
		bp.Bytes = nil
		return
	}
	bp.Bytes = bp.Bytes[:0]
	pool.Put(bp)
}

// --- 向后兼容的 Get/Put（基于 Buffer Lease 实现） ---
//
// 旧 API 保留给尚未迁移的调用方。Get 返回 []byte，Put 归还。由于 Get 丢失了与
// *Buffer 的关联，Put 通过把切片重新包装为 *Buffer 归还——这会引入一次小的指针分配
// （指向栈上切片的指针逃逸），因此稳态仍有 1 alloc。新代码应优先用 Acquire/Release
// 获得 0 分配。

// Get 返回长度为 n 的字节切片。向后兼容接口；新代码用 Acquire。
func Get(n int) []byte {
	return Acquire(n).Bytes
}

// Put 归还 Get 得到的切片。向后兼容接口；新代码用 Release。
func Put(b []byte) {
	Release(&Buffer{Bytes: b})
}
