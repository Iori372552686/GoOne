package crypto

import (
	"GoOne/lib/util/convert"
	"encoding/base64"
)

var coder = base64.StdEncoding

/**
* @Description: base64加密
* @param: src
* @return: []byte
* @Author: Iori
* @Date: 2022-06-07 18:41:19
**/
func Base64Encode(src []byte) []byte {
	return []byte(coder.EncodeToString(src))
}

/**
* @Description: base64解密
* @param: src
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-07 18:41:24
**/
func Base64Decode(src []byte) ([]byte, error) {
	return coder.DecodeString(convert.Bytes2str(src))
}

/**
* @Description: base64加密 str
* @param: src
* @return: []byte
* @Author: Iori
* @Date: 2022-06-07 18:41:19
**/
func Base64EncodeStr(src []byte) string {
	return coder.EncodeToString(src)
}

/**
* @Description: base64解密  str
* @param: src
* @return: []byte
* @return: error
* @Author: Iori
* @Date: 2022-06-07 18:41:24
**/
func Base64DecodeStr(src string) ([]byte, error) {
	return coder.DecodeString(src)
}
