package tester

import (
	"fmt"
	"testing"
	"time"
)

func fn1(t *testing.T) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <- timer.C:
			break
		}
		fmt.Println("aaaa")
		break
	}
	fmt.Println("bbb")
}

func fn2(t *testing.T) {
	fmt.Println("a")
	if true {
		defer fmt.Println("b")
	}
	fmt.Println("c")
}

func fn3() {
	for _ = range "12" {
		fmt.Println("a")
	}
}

func TestT(t *testing.T) {
	fn3()
}
