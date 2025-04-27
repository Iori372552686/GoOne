package random

import (
	encoding_binary "encoding/binary"
	"io"
	math_rand "math/rand"
	"os"
	"runtime"
	"time"
)

func init() {
	var seed uint64
	if runtime.GOOS == "linux" {
		fp, err := os.Open("/dev/urandom")
		if err != nil {
			panic(err)
		}
		defer fp.Close()
		bytes := make([]byte, 8)
		if _, err = io.ReadFull(fp, bytes); err != nil {
			panic(err)
		}
		seed = encoding_binary.LittleEndian.Uint64(bytes)
	} else {
		seed = uint64(time.Now().Nanosecond())
	}
	math_rand.Seed(int64(seed))
}
