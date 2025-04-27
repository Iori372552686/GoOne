package random

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetString(t *testing.T) {
	for i := 0; i < 10; i++ {
		t.Log(GetString(20))
	}
}

func TestGetBytes(t *testing.T) {
	for i := 0; i < 10; i++ {
		bts := GetBytes(20)
		t.Log(bts)
		if len(bts) != 20 {
			panic("not equal")
		}
	}
	bts := GetBytes(-1)
	assert.Nil(t, bts)

	bts = GetBytes(2048)
	assert.Nil(t, bts)

	bts = GetBytes(1024)
	assert.Equal(t, 1024, len(bts))

}

func TestParallelRand(t *testing.T) {
	var wg sync.WaitGroup
	const workerNum = 100
	wg.Add(workerNum)
	for i := 0; i < workerNum; i++ {
		go func() {
			for j := 0; j < 100000; j++ {
				_ = Int()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestUint(t *testing.T) {
	ret := Uint64()
	assert.True(t, ret > 0)
}

func TestLimit(t *testing.T) {
	ret := Intn(100)
	assert.True(t, ret < 100 && ret > 0)
}
