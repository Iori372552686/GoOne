package sharedstruct

import (
	"fmt"
	"testing"
	"unsafe"
)

func TestCap(t *testing.T) {
	fmt.Println(unsafe.Sizeof(SSPacketHeader{}))
}
