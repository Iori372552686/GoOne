package zip

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestGzip(b *testing.T) {

	str := "hello test"
	data := Gzip([]byte(str))
	data1, _ := GzipEncode([]byte(str))
	fmt.Println([]byte(data))
	fmt.Println(data1)

	msg, err := GzipDecode([]byte(data[:18]))
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(msg))

	msg1, err1 := UnGzip(data1)
	if err1 != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(msg1)

}

func TestGzip2(t *testing.T) {
	data := []byte("helloworld")

	binary, _ := GzipEncode(data)
	fmt.Println(binary)
	fmt.Println(hex.EncodeToString(binary))
	binary = ZipEncode(data)
	fmt.Println(binary)

	res, err := GzipDecode(binary)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(res))
}

func TestGzip3(t *testing.T) {
	hexStr := "1f8b08000000000000004d52db6ee4200cfd17f23aa9b8e3ccebbeef2f202ea61335256d42a76a57fbef6b9a9d4e918e908f0fe660b3ac8f3ee3754e380c7f185eb136dfe667646761ad56465bcb4f3ff8cfb5524e03913beefbbc563f677666454f935045262d8d36dc4170d94c41151489566427765c73c8b3741324eb4c02873ac8ac21f0508a988271b274792887f4e6436829c4a88ca1a3640a2cd7d28272b26bf397343ac8d2ea692c25eb51bb64c6186d1c93b013979090cc74f5cb8bbfe2d6cdf7fa0ffcc172a2d325d48a0b518aab1eaf6fb56d1f7edd89af6fb56d1f7edd89faf59b9825d4c723fcbc8c7786ea11d5ded9ad5169c3d0d0a7756fec2c2775af357725e96efb9e9f7c0dbddd2c44df706fffa7e1c3b2507e6d17dca80f7eae65edcee46060707600370849d030086309d320ac2028822150ce51ec287694034e9004dd6fc59afd8eaf34e4136b212e7833b17c7f87fb63be5ea1843cb10d5fdfbac563e2321628067930a8255840305964542030c5acd8df7f"

	data, _ := hex.DecodeString(hexStr)
	fmt.Println(data)

	res, err := GzipDecode(data)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(res))
}
