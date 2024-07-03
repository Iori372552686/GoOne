package convert

import (
	"bytes"
	"encoding/binary"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/bytedance/sonic/decoder"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/bytedance/sonic/encoder"
)

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)

	var x int32
	binary.Read(bytesBuffer, binary.BigEndian, &x)

	return int(x)
}

func Str2bytes(s string) (b []byte) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pbytes.Data = pstring.Data
	pbytes.Len = pstring.Len
	pbytes.Cap = pstring.Len
	return
}

func Bytes2str(b []byte) (s string) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pstring.Data = pbytes.Data
	pstring.Len = pbytes.Len
	return
}

func IntToString(i int) string {
	return strconv.Itoa(i)
}

func Int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}

func StrToInt(num string) int {
	i, _ := strconv.Atoi(num)
	return i
}

func StrToInt32(num string) int32 {
	i, _ := strconv.Atoi(num)
	return int32(i)
}

/**
* @Description: st to json str use https://github.com/bytedance/sonic library
* @param: st
* @return: string
**/
func StructToJsonStr(st interface{}) string {
	var data = bytes.NewBuffer(nil)

	err := encoder.NewStreamEncoder(data).Encode(st)
	if err != nil {
		logger.Errorf("StructToJsonStr: %v", err.Error())
		return "{}"
	}

	return data.String()
}

/**
* @Description:   st to json use https://github.com/bytedance/sonic library
* @param: st_
* @return: []byte
**/
func StructToJson(st interface{}) []byte {
	var data = bytes.NewBuffer(nil)

	err := encoder.NewStreamEncoder(data).Encode(st)
	if err != nil {
		logger.Errorf("StructToJson: %v", err.Error())
		return nil
	}

	return data.Bytes()
}

/**
* @Description: json to st  use https://github.com/bytedance/sonic library
* @param: jsStr
* @param: st
* @return: error
* @Author: Iori
**/
func JsonToStruct(jsStr string, st interface{}) error {
	deCoder := decoder.NewStreamDecoder(strings.NewReader(jsStr))
	err := deCoder.Decode(st)
	if err != nil {
		logger.Errorf("convert (%v) to json string error:%v", st, err)
		return err
	}
	return nil
}

/**
* @Description: use  https://github.com/bytedance/sonic  library to json Encode
* @param: hashmap
* @return: string
* @Author: Iori
**/
func MapToJson(hashmap interface{}) string {
	var w = bytes.NewBuffer(nil)

	encoder.NewStreamEncoder(w).Encode(hashmap)
	return w.String()
}

/**
* @Description:  use  https://github.com/bytedance/sonic  library to json Decode
* @param: body
* @return: *map[string]interface{}
* @return: error
* @Author: Iori
**/
func JsonToMap(body []byte) (*map[string]interface{}, error) {
	var decMap map[string]interface{}

	return &decMap, decoder.NewStreamDecoder(bytes.NewBuffer(body)).Decode(&decMap)
}

/**
* @Description: json 转 map interface , 数值使用https://github.com/bytedance/sonic number 版本
* @param: body
* @param: dst
* @return: error
* @Author: Iori
**/
func JsonToMapUseNumber(body []byte, dst *map[string]interface{}) error {
	dec := decoder.NewStreamDecoder(bytes.NewBuffer(body))
	dec.UseNumber()

	return dec.Decode(dst)
}

/**
* @Description:   json 转 map interface , 数值使用https://github.com/bytedance/sonic int64版本
* @param: body
* @param: dst
* @return: error
* @Author: Iori
**/
func JsonToMapUseInt64(body []byte, dst *map[string]interface{}) error {
	dec := decoder.NewStreamDecoder(bytes.NewBuffer(body))
	dec.UseInt64()

	return dec.Decode(dst)
}
