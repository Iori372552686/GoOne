package random

import (
	"encoding/binary"
	"math/rand"
)

// GetBytes 最多返回1~1024 字节长度的随机bytes
func GetBytes(size int) []byte {
	if size <= 0 || size > 1024 {
		return nil
	}
	buf := make([]byte, size)
	n := size / 8
	r := size % 8
	bs := n * 8
	for i := 0; i < n; i++ {
		rd := rand.Uint64()
		binary.BigEndian.PutUint64(buf[i*8:], rd)
	}

	for i := 0; i < r; i++ {
		buf[bs+i] = byte(rand.Uint64() % 0xff)
	}
	return buf
}
