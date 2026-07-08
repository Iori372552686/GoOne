package safego

import (
	"sync"
	"testing"
)

// TestSafeFuncRecoversPanic verifies that a panic inside SafeFunc is recovered
// and does not terminate the process.
func TestSafeFuncRecoversPanic(t *testing.T) {
	SafeFunc(func() {
		var m map[string]string
		m["k"] = "v" // panics: assignment to entry in nil map
	})
	// Reaching here means the panic was recovered.
}

func TestSafeFuncNil(t *testing.T) {
	SafeFunc(func() {})
	Go(nil) // must not panic
}

func TestGoRunsFunc(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	Go(func() {
		defer wg.Done()
	})
	wg.Wait()
}

// TestGoRecoversPanic verifies that a panicking goroutine spawned via Go does
// not crash the process.
func TestGoRecoversPanic(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	Go(func() {
		defer wg.Done()
		panic("boom")
	})
	wg.Wait()
}
