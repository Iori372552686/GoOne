package safego

import (
	"context"
	"strconv"
	"sync"
	"testing"
)

func TestSafeMap(t *testing.T) {
	var m map[string]string
	SafeFunc(context.Background(), func(c context.Context) {
		m["k"] = "v"
	})
	m = make(map[string]string)
	wg := sync.WaitGroup{}
	wg.Add(2)
	Go(context.Background(), func(c context.Context) {
		for i := 0; i < 1000000; i++ {
			m["k"] = strconv.Itoa(i)
		}
		wg.Done()
	})
	Go(context.Background(), func(c context.Context) {
		for i := 0; i < 1000000; i++ {
			m["k"] = strconv.Itoa(i)
		}
		wg.Done()
	})
	wg.Wait()
	t.Log("safe")
}
