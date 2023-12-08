package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"github.com/Iori372552686/GoOne/lib/util/crypto"
)

/**
 * ecb
 * @Description:
**/
type Ecb struct {
	b         cipher.Block
	blockSize int
	//iv        []byte   ecb 模式無需iv向量
}

func newECB(b cipher.Block) *Ecb {
	return &Ecb{
		b:         b,
		blockSize: b.BlockSize(),
	}
}

type ecbEncrypter Ecb
type ecbDecrypter Ecb

/**
* @Description:   Ecb encrypter
* @param: b
* @return: cipher.BlockMode
* @Author: Iori
* @Date: 2023-03-06 17:41:46
**/
func NewECBEncrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbEncrypter)(newECB(b))
}
func (x *ecbEncrypter) BlockSize() int { return x.blockSize }
func (x *ecbEncrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	for len(src) > 0 {
		x.b.Encrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

/**
* @Description: Ecb decrypter
* @param: b
* @return: cipher.BlockMode
* @Author: Iori
* @Date: 2023-03-06 17:41:46
**/
func NewECBDecrypter(b cipher.Block) cipher.BlockMode {
	return (*ecbDecrypter)(newECB(b))
}

func (x *ecbDecrypter) BlockSize() int { return x.blockSize }
func (x *ecbDecrypter) CryptBlocks(dst, src []byte) {
	if len(src)%x.blockSize != 0 {
		panic("crypto/cipher: input not full blocks")
	}
	if len(dst) < len(src) {
		panic("crypto/cipher: output smaller than input")
	}
	for len(src) > 0 {
		x.b.Decrypt(dst, src[:x.blockSize])
		src = src[x.blockSize:]
		dst = dst[x.blockSize:]
	}
}

//---------------------------------------------------------------- func

/**
* @Description:  Ecb  encryption 128bit
* @param: encryptStr
* @param: key
* @param: iv
* @return: string
* @return: error
* @Author: Iori
* @Date: 2023-03-06 17:02:27
**/
func EcbEncrypt(encrypt, key string) (string, error) {
	encryptBytes := convert.Str2bytes(encrypt)
	block, err := aes.NewCipher(convert.Str2bytes(key))
	if err != nil {
		return "", err
	}

	encryptBytes = pkcs7Padding(encryptBytes, block.BlockSize())
	blockMode := NewECBEncrypter(block)
	encrypted := make([]byte, len(encryptBytes))
	blockMode.CryptBlocks(encrypted, encryptBytes)
	return crypto.Base64EncodeStr(encrypted), nil
}

/**
* @Description:   Ecb  decrypt
* @param: decryptStr
* @param: key
* @param: iv
* @return: string
* @return: error
* @Author: Iori
* @Date: 2023-03-06 17:27:21
**/
func EcbDecrypt(decryptStr, key string) (string, error) {
	decryptBytes, err := crypto.Base64DecodeStr(decryptStr)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(convert.Str2bytes(key))
	if err != nil {
		return "", err
	}

	blockMode := NewECBDecrypter(block)
	decrypted := make([]byte, len(decryptBytes))
	blockMode.CryptBlocks(decrypted, decryptBytes)
	decrypted = pkcs7UnPadding(decrypted)
	return convert.Bytes2str(decrypted), nil
}
