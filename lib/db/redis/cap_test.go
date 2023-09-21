package redis

import (
	"fmt"
	"testing"
	"time"
)

func TestCap(t *testing.T) {
	const c = 1 * 1024

	b := [c]byte{}
	for i := 0; i < c; i++ {
		b[i] = byte(i % 256)
	}

	redisMgr := NewRedisMgr()
	err := redisMgr.AddInstance(1, "10.0.0.173", 6379, "mWtiidKGE6Bb8esnFB8", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for i := 0; i < 100; i++ {
		err = redisMgr.SetBytes(1, fmt.Sprintf("0x%08x", i), b[:])
		if err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println(time.Since(now))
}

func TestIncBy(t *testing.T) {

	redisMgr := NewRedisMgr()
	err := redisMgr.AddInstance(1, "10.0.0.173", 6379, "mWtiidKGE6Bb8esnFB8", 0, false)
	if err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 24; i++ {
		ret, err := redisMgr.IncrByKey(1, "IncrTest2", 2)
		if err != nil {
			t.Fatal(err)
		}

		fmt.Printf("ret =%v\n", ret)
	}
}
