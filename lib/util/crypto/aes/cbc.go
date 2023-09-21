package aes

import (
	"GoOne/lib/util/convert"
	"GoOne/lib/util/crypto"
	"crypto/aes"
	"crypto/cipher"
)

/*
*
  - @Description:  cbc  encryption 128bit
  - @param: encryptStr
  - @param: key
  - @param: iv
  - @return: string
  - @return: error
  - @Author: Iori
    2023-03-06 17:02:27

*
*/
func CbcEncrypt(encrypt, key, iv string) (string, error) {
	encryptBytes := convert.Str2bytes(encrypt)
	block, err := aes.NewCipher(convert.Str2bytes(key))
	if err != nil {
		return "", err
	}

	encryptBytes = pkcs7Padding(encryptBytes, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, convert.Str2bytes(iv))
	encrypted := make([]byte, len(encryptBytes))
	blockMode.CryptBlocks(encrypted, encryptBytes)
	return crypto.Base64EncodeStr(encrypted), nil
}

/*
*
  - @Description:  cbc decrypt
  - @param: decryptStr
  - @param: key
  - @param: iv
  - @return: string
  - @return: error
  - @Author: Iori
    2023-03-06 17:04:03

*
*/
func CbcDecrypt(decryptStr string, key, iv string) (string, error) {
	decryptBytes, err := crypto.Base64DecodeStr(decryptStr)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(convert.Str2bytes(key))
	if err != nil {
		return "", err
	}

	blockMode := cipher.NewCBCDecrypter(block, convert.Str2bytes(iv))
	decrypted := make([]byte, len(decryptBytes))
	blockMode.CryptBlocks(decrypted, decryptBytes)
	decrypted = pkcs7UnPadding(decrypted)
	return convert.Bytes2str(decrypted), nil
}
