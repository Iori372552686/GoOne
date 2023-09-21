package aes

import (
	"bytes"
)

/**
* @Description: pkcs 5 Padding
* @param: cipherText
* @param: blockSize
* @return: []byte
* @Author: Iori
* @Date: 2023-03-06 17:21:39
**/
func pkcs5Padding(cipherText []byte, blockSize int) []byte {
	padding := blockSize - len(cipherText)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(cipherText, padText...)
}

func pkcs5UnPadding(decrypted []byte) []byte {
	length := len(decrypted)
	unPadding := int(decrypted[length-1])
	return decrypted[:(length - unPadding)]
}

/**
* @Description: pkcs 7 Padding   使用PKCS7进行填充，IOS也是7
* @param: data
* @return: []byte
* @Author: Iori
* @Date: 2023-03-06 17:22:05
**/
func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func pkcs7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}
