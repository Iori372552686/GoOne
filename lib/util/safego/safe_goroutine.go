package safego

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"runtime/debug"
	"time"
)

// Go runs a safe goroutine
func Go(f func()) {
	if f == nil {
		return
	}

	go SafeFunc(f)
}

// SafeFunc safe function call
func SafeFunc(f func()) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			fmt.Println(time.Now().String())
			fmt.Println(r)
			fmt.Println(stack)
			logger.Fatalf("%v : %s ", r, stack)
		}
	}()
	f()
}
