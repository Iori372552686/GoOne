package lru

import (
	"sync"
	"testing"
)

func TestLRUCache_BasicSetGet(t *testing.T) {
	lru := NewLRUCache(2)
	if err := lru.Set("a", 1); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if err := lru.Set("b", 2); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if v, ok, err := lru.Get("a"); err != nil || !ok || v.(int) != 1 {
		t.Fatalf("Get a failed, v=%v ok=%v err=%v", v, ok, err)
	}
}

func TestLRUCache_EvictionOrder(t *testing.T) {
	lru := NewLRUCache(2)
	_ = lru.Set("a", 1)
	_ = lru.Set("b", 2)

	// access a so that b becomes LRU
	if _, ok, _ := lru.Get("a"); !ok {
		t.Fatalf("expected to find key a")
	}

	// adding c should evict b
	_ = lru.Set("c", 3)

	if _, ok, _ := lru.Get("b"); ok {
		t.Fatalf("expected key b to be evicted")
	}
	if v, ok, _ := lru.Get("a"); !ok || v.(int) != 1 {
		t.Fatalf("expected key a to remain with value 1, got v=%v ok=%v", v, ok)
	}
	if v, ok, _ := lru.Get("c"); !ok || v.(int) != 3 {
		t.Fatalf("expected key c to exist with value 3, got v=%v ok=%v", v, ok)
	}
}

func TestLRUCache_Remove(t *testing.T) {
	lru := NewLRUCache(2)
	_ = lru.Set("a", 1)
	_ = lru.Set("b", 2)

	if !lru.Remove("a") {
		t.Fatalf("expected remove a to return true")
	}
	if _, ok, _ := lru.Get("a"); ok {
		t.Fatalf("expected a to be removed")
	}
	if size := lru.Size(); size != 1 {
		t.Fatalf("expected size 1 after remove, got %d", size)
	}
}

func TestLRUCache_ZeroCapacity(t *testing.T) {
	lru := NewLRUCache(0)
	_ = lru.Set("a", 1)
	_ = lru.Set("b", 2)

	if size := lru.Size(); size != 1 {
		t.Fatalf("expected size 1 with adjusted capacity, got %d", size)
	}
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	lru := NewLRUCache(100)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				key := id*1000 + j
				_ = lru.Set(key, j)
				_, _, _ = lru.Get(key)
			}
		}(i)
	}
	wg.Wait()

	// basic sanity: no panic and Size within capacity
	if lru.Size() > int(lru.Capacity) {
		t.Fatalf("size should not exceed capacity, got %d", lru.Size())
	}
}

func TestLRUCache_PeekDoesNotAffectOrder(t *testing.T) {
	lru := NewLRUCache(2)
	_ = lru.Set("a", 1)
	_ = lru.Set("b", 2)

	// After Set(a) then Set(b), key a is the least recently used.
	// Peek must NOT promote a, so the next eviction still removes a.
	if v, ok, err := lru.Peek("a"); err != nil || !ok || v.(int) != 1 {
		t.Fatalf("Peek a failed, v=%v ok=%v err=%v", v, ok, err)
	}

	// Set c: evicts a (LRU), because Peek(a) did not update its order.
	_ = lru.Set("c", 3)

	if _, ok, _ := lru.Get("a"); ok {
		t.Fatalf("expected key a to be evicted after Peek(a) and Set(c)")
	}
	if v, ok, _ := lru.Get("b"); !ok || v.(int) != 2 {
		t.Fatalf("expected key b to remain with value 2, got v=%v ok=%v", v, ok)
	}
	if v, ok, _ := lru.Get("c"); !ok || v.(int) != 3 {
		t.Fatalf("expected key c to exist with value 3, got v=%v ok=%v", v, ok)
	}
}
