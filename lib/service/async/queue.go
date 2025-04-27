package async

import (
	"sync/atomic"
	"unsafe"
)

type node struct {
	next  *node
	value interface{}
}

type Queue struct {
	head  *node
	tail  *node
	count int64
}

func NewQueuePool(size int64) (rets []*Queue) {
	rets = make([]*Queue, size)
	for i := int64(0); i < size; i++ {
		rets[i] = NewQueue()
	}
	return
}

func NewQueue() *Queue {
	node := new(node)
	return &Queue{head: node, tail: node}
}

// 多协程安全
func (d *Queue) Push(val interface{}) {
	addNode := new(node)
	addNode.value = val
	// 将新增节点插入链表
	prevNode := (*node)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&d.tail)), unsafe.Pointer(addNode)))
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&prevNode.next)), unsafe.Pointer(addNode))
	atomic.AddInt64(&d.count, 1)
}

// 单协程安全
func (d *Queue) Pop() (ret interface{}) {
	if node := (*node)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&d.head.next)))); node != nil {
		atomic.AddInt64(&d.count, -1)
		ret = node.value
		d.head.next = nil
		d.head = node
	}
	return
}

func (d *Queue) GetCount() int64 {
	return atomic.LoadInt64(&d.count)
}
