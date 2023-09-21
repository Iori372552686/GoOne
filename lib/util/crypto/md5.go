package crypto

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
)

/**
* @Description: 正常md5
* @param: str
* @return: string
* @Author: Iori
**/
func Md5(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

/**
* @Description:  md5全小写str
* @param: arg0
* @return: string
* @Author: Iori
**/
func Md5Encode(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return strings.ToLower(hex.EncodeToString(h.Sum(nil)))
}

/**
* @Description:  md5全大写str
* @param: arg0
* @return: string
* @Author: Iori
**/
func Md5EncodeV2(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}
