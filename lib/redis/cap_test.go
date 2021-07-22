package redis

import (
	"fmt"
	"testing"
)

func TestCap(t *testing.T) {


	const c = 1024*1024
	b := [c]byte{}
	for i:= 0; i < c; i++ {
		b[i] = byte(i % 256)
	}




	redisMgr := NewRedisMgr()
	err := redisMgr.AddInstance(1, "192.168.2.61", 6379, "rdpw", 0, false)
	if err != nil {
		t.Fatal(err)
	}

	for i:= 1024*2; i < 1024*3; i++ {
		err = redisMgr.SetBytes(1, fmt.Sprintf("0x%08x", i), b[:])
		if err != nil {
			t.Fatal(err)
		}
	}
}