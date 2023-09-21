package marshal

import (
	"fmt"
	"os"
	"testing"

	"github.com/vmihailenco/msgpack"
)

func Test_msgpack(t *testing.T) {
	countryCapitalMap := make(map[string]string)
	out := make(map[string]string)
	countryCapitalMap["device_id"] = "345"
	countryCapitalMap["session_id"] = "123"

	test, err := os.ReadFile("d:/test.log")

	in := countryCapitalMap
	res, err := msgpack.Marshal(in)
	if err != nil {
		fmt.Printf("序列化失败")
	}

	fmt.Printf("原数据byte=%v", res)
	fmt.Printf("test byte=%v", test)

	err = msgpack.Unmarshal(test, &out)
	if err != nil {
		fmt.Println("反序列化失败")
	}
	fmt.Println("反序列化数据--", out)

}
