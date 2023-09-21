package crypto

import (
	"GoOne/lib/util/crypto/aes"
	"GoOne/lib/util/crypto/xxtea"
	"fmt"
	"log"
	"testing"
)

var data = []byte("xxtea-go test case")
var str = "4OdYH86kbYUA2wrNTC3hT/zqb4bUWlnZ++1e+ws0sgJFBS8Iu6lhgtnc5q5NCMt10T40Pe5sys2n5A3VTaG2uXJsxwxoyiGaelgnzi5EGrRMFm0WeIExUNF7ld54b7dw5pBPzuzC8fQw7RsnlPtVfVzbUH4xm5p5o5+0ptDAhfUXOMHX6uxMDEr++IrAgzp6H6FnY65G7kfoJ+iIl0RJOlhYN3Jvs9yhhsfr5blwA3kQOjvo5gTGw5ZBEdliGIYRCRV0TfAf++daZecXVm6sfylPtSFn7+4K9kpkYmh+9ZnPUxyvIKnpGultmqnHyDMgmrpWxbV/ZULVy+/x+kgzgs2HtVBDlfojAeVzFI2PsoEiFt396FTsJOt2p5cpqJplBl3VaxaJr+gPmPdIHKZEmr15oeId/9CvzKMbHW2LdeHO+AaYmBcqClGpBkdhrjshVn8qO6Q39hnnXVak0ye118Gb/niTldr7Fr27/vELuhYGUwx0STmx/qQloat778VT8L1OV/Fwf2DC7S/JKHV+OBhBz3S5yEWm+02XUQZG0wA4rXKZsvYOXQFQ8Vj5VWiV3TAbst4CLtiKVCQZIKCG2Q67XquLGO8Ws/TUp48e/QIETF2XBsEkiHVChgMMdQmHtbywDc6eEpQwFnGI0V6hBNpigu2opDGksOs1eBovHOGzKVVgOsJfaAG4ISyE9R5hMbCae4fJM7pmAd7t8kX4coL0qLx2+IopZCCdlDVXx+l4VbpVs/0uy1edFXFPeWGTfN9SaS1qBcRaJk/3z7fzWIzQf9fDhYEb9rsFRQTIFdWiCT54"
var key = []byte{0xb4, 0x8e, 0x6e, 0xf4, 0x4e, 0xd1, 0x3e, 0xee, 0x60, 0x61, 0x41, 0x75, 0x0e, 0x72, 0x9c, 0xf4}
var str1 = "tJPn2ROoWSXSjvkOfTrWP+ElWgKWea24C2g/EPsKJwnpZxck7nxttI5lb355EpnW3foY3KjeDnwzm75o0xhoayBMAXR1v0pFSRszxhpUIyjK4SaE7sLkH1UM6pl9fxti+a3fbxHNOyrLAz2qLvhoNsgV+c9JXDuPrw1NpIgDmZ7gdpKEIy+fYocOleOLNy2P2nsb5zv69jmxmwiqZUpHSd/ulw20Dqtb5rhjMJyDxodbgAxA3uoByBAZDPtgBoAXVqfawaLeYNr/e6J45C9jDpJicZvpPWtDJPGXQR8JT0bJbeGAPtfaubjRPX67/O5eJ6+pPWtI1/OuD+nHySo3EjPMIK4Mxz7FBAaOKlc8Fgi6Gk7uEyrrYebuKBQbQQS6HvDeSD4OuX0pOX/xv9Kx90IYX6ihYzYauKdzMDja+zNtZlij082u8UtWXIdtlDVblZXdX+oARh7xNGWbQF1J7gjMWNrqCn5HnbbsODl4Fm3acOhq65pr54wCs/PKX+GOfdIdFFxO1qGkurc8JPTwPNTG+psVrYwdJ1FX2Kr0JYFrjg2ILSz2h4AxASoHkxprvd6agOAOfUwSQAx8yWg+W6mdpMrjfqicGWT7LRLvveLowCCbk5RS5d2rKp+IC3DD3DNVoSTejl52B9gFqFNXPAHk9ACCNXjQk4KlflGB4rKuonik95d+eme5VHY5SgJ7gTRinLxTgDuZ0QHobsApKsQuNcZx5M+X1nrv8zH/vNE73x2kjc9T/y1waZaY0+Iz3g7r1j8hF4RjuLnYxUX48kDZgeIRBJIMlRXIjnuxZYMbM4yRsHByiXdG7wLt6ySIE+vBFqSyCiy5ts9K9Cep9Cdq8ySsn2en8CXq0qHq/ev4jByVUVFgVEgNRTx9UVvTkNUF18PXwBeuG1Zf42XdT6eLzJ7xqTtYf/GJajdX8NNjqFzEa7YI0yqWrR8LULD61yvn2i1JavyuEGwyPeIkba9BaEBjtIk19y70z9rc/JLG0UiUBFoh+DkWzyl9MzrQbrfTFHC0JbbiSK8LPmIuSQ9hfo/I3zoElG0cqG8Wq61Ryk0oSQ2cIFS7u8HCAKmWyv2Prt029yPU4zHFw4qLZnMPt9KTnMGEoPZF1F0yToUXWwYX0/zcdV5QetQZSgReBh8uqWTvD3PqKN/4jgdY26xPJzAQ2rAIJFIAoH1hV0THnpJzE2pzi3CkgrYyS7w+UdZQEPkv2rwyso5YO9eFgNkH8VYIiNx8DG9OsTvbRsqsmot0H33P2qeKzQCJG2yqE23kh6hR9TSbdwT6wzMfY/socJx7UeV0tmc1gkigroqDnfs4AbIUAYpFgyi4kSQlwra8ww1iPq72/oKa/eK5JS9R8THTln09FiyhSt5NaRFxEH/wcl0f5gtedlywoy8RouweJt842YNnbx0Ew7k8ijJ2CdvZaGpbE5UH07GE+Gnzdmf6vu7qARKIdnkfw7iVO57yASjK3pPz8httrs+KPp91BBsvYpvm2GRN95pZCaD4Jh+gDik819Tee1oOf6MjqQlfKlJf506"

func TestXtDecode(t *testing.T) {

	str1, err := xxtea.DecryptBase64(str1, key, false, 0)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(str1))
}

func TestEncrypt(t *testing.T) {
	enc, err := xxtea.Encrypt(data, key, true, 0)
	if err != nil {
		t.Error(err)
	}

	str1, err := xxtea.Decrypt(enc, key, true, 0)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(str1))
}

func TestEncryptBase64(t *testing.T) {
	b64Enc, err := xxtea.EncryptBase64(data, key, true, 0)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(string(b64Enc))

	data1, err := xxtea.DecryptBase64(b64Enc, key, true, 0)
	if err != nil {
		t.Error(err)
	}

	fmt.Println(string(data1))
}

func TestB64(b *testing.T) {

	str := "test"
	data := Base64Encode([]byte(str))
	fmt.Println(data)

	msg, _ := Base64Decode(data)
	fmt.Println(string(msg))
}

func TestAes(b *testing.T) {
	origData := "Hello World" // 待加密的数据
	key := "ABCDEFGHIJKLMNOP" // 加密的密钥
	iv := "!xxxxx20wbxxxx#z"  // 偏移的向量iv
	log.Println("原文：", string(origData))

	log.Println("------------------ CBC模式 --------------------")
	encrypted, _ := aes.CbcEncrypt(origData, key, iv)
	//log.Println("密文(hex)：", hex.EncodeToString([]byte(encrypted)))
	log.Println("密文(base64)：", encrypted)
	decrypted, _ := aes.CbcDecrypt(encrypted, key, iv)
	log.Println("解密结果：", decrypted)

	log.Println("------------------ ECB模式 --------------------")
	encrypted, _ = aes.EcbEncrypt(origData, key)
	//log.Println("密文(hex)：", hex.EncodeToString(encrypted))
	log.Println("密文(base64)：", encrypted)
	decrypted, _ = aes.EcbDecrypt(encrypted, key)
	log.Println("解密结果：", string(decrypted))

	log.Println("------------------ CFB模式 --------------------")
	encrypted, _ = aes.CfbEncrypt(origData, key, iv)
	//log.Println("密文(hex)：", hex.EncodeToString(encrypted))
	log.Println("密文(base64)：", encrypted)
	decrypted, _ = aes.CfbDecrypt(encrypted, key, iv)
	log.Println("解密结果：", decrypted)
}
