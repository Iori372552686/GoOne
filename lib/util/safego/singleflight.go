package safego

import (
	"context"
	"sync"
)

//代表正在进行中，或已经结束的请求
type call struct {
	sync.WaitGroup //避免重入：可能在f调用期间有n次调用，但是只执行一次
	val            interface{}
	err            error
}

//SingleFlight 管理不同key的请求
type SingleFlight struct {
	lock sync.Mutex //保护m并发读写
	m    map[string]*call
}

// Do 针对相同的key，无论Do被调用多少次，f只执行一次
func (g *SingleFlight) Do(ctx context.Context, key string, f func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	g.lock.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	//如果已有结果，返回
	if c, ok := g.m[key]; ok {
		g.lock.Unlock()
		c.Wait() //等待f调用结束了，再返回
		return c.val, c.err
	}
	c := new(call)
	c.Add(1)
	g.m[key] = c //缓存执行结果
	g.lock.Unlock()

	//调用函数
	SafeFunc(func() {
		c.val, c.err = f(ctx)
	})

	c.Done()

	g.lock.Lock()
	delete(g.m, key)
	defer g.lock.Unlock()

	return c.val, c.err
}
