package common

import (
	"GoOne/lib/api/logger"
	"GoOne/lib/util/convert"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

/**
* @Description: 检测 recover并打印错误 stack
* @Date: 2022-02-19 10:33:49
**/
func CheckRecover() {
	err := recover()
	if err != nil {
		buf := make([]byte, 4096)
		buf = buf[:runtime.Stack(buf, false)]
		logger.Errorf("!!!!!!!!err: %v\n===============\n%s", err, buf)
	}
}

/**
* @Description: 检测 recover并跟踪代码
* @Date: 2022-02-26 18:00:12
**/
func RecoverTraceCode() {
	err := recover()
	if err != nil {
		TraceCode(err)
	}
}

/**
* @Description: 输出错误，跟踪代码
* @param: code
* @Author: Iori
* @Date: 2022-02-26 18:00:28
**/
func TraceCode(code ...interface{}) {
	buf := make([]byte, 4096)

	n := runtime.Stack(buf[:], false)
	data := ""
	for _, v := range code {
		data += fmt.Sprintf("%v", v)
	}

	buf = buf[:n]
	logger.Errorf("==> err: %v\n===============\n%s\n", data, buf)
}

/**
* @Description: 获取当前协程id
* @return: uint64
* @Author: Iori
* @Date: 2022-02-19 10:32:39
**/
func GetGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

/**
* @Description: 结构体数据复制
* @param: binding  要修改的结构体
* @param: value  有数据的结构体
* @Author: Iori
* @Date: 2022-02-19 10:32:57
**/
func StructAssign(binding interface{}, value interface{}) {
	bVal := reflect.ValueOf(binding).Elem()
	vVal := reflect.ValueOf(value).Elem()
	vTypeOfT := vVal.Type()
	for i := 0; i < vVal.NumField(); i++ {
		name := vTypeOfT.Field(i).Name
		if ok := bVal.FieldByName(name).IsValid(); ok {
			bVal.FieldByName(name).Set(reflect.ValueOf(vVal.Field(i).Interface()))
		}
	}
}

/**
* @Description: 随机指定长度字符串
* @param: n
* @return: string
* @Author: Iori
* @Date: 2022-06-01 15:19:58
**/
func GetRandomStr(n int) string {
	bytes := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	var result []byte

	for i := 0; i < n; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return convert.Bytes2str(result)
}

/**
* @Description:
* @param: n
* @return: string
* @Author: Iori
* @Date: 2022-06-01 16:41:37
**/
func GetRandomByte(n int) []byte {
	str := "0123456789abcdefghijklmnopqrstuvwxyz"
	bytes := []byte(str)
	var result []byte
	for i := 0; i < n; i++ {
		result = append(result, bytes[rand.Intn(len(bytes))])
	}
	return result
}

func isInclude(list *[]string, val string) bool {
	if list == nil {
		return false
	}

	for _, a := range *list {
		if a == val {
			return true
		}
	}
	return false
}

func CheckCountryZone(country, zone string, en_country, en_zone, dis_country, dis_zone *[]string) bool {
	country = strings.ToUpper(country)
	zone = strings.ToUpper(zone)

	//特殊处理
	if country == "CN" || country == "" {
		return true
	}
	// 如果没有配置包含、排除国家和大区，默认符合
	if (en_country == nil || len(*en_country) == 0) &&
		(dis_country == nil || len(*dis_country) == 0) &&
		(en_zone == nil || len(*en_zone) == 0) &&
		(dis_zone == nil || len(*dis_zone) == 0) {
		return true
	}

	//如果当前国家或者大区在排除列表中，返回不符合
	if isInclude(dis_country, country) || isInclude(dis_zone, zone) {
		return false
	}

	// 如果当前国家在包含列表中，并且大区为空，返回符合
	if isInclude(en_country, country) && (zone == "") {
		return true
	}

	//如果当前国家在包含列表中，并且大区不为空，也包含在列表中，返回符合
	if isInclude(en_country, country) && zone != "" && isInclude(en_zone, zone) {
		return true
	}

	return true
}

/**
* @Description:  获取本地ip地址
* @return: ip
* @return: err
**/
func GetLocalIP() (ip string, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}
	for _, addr := range addrs {
		ipAddr, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipAddr.IP.IsLoopback() {
			continue
		}
		if !ipAddr.IP.IsGlobalUnicast() {
			continue
		}
		return ipAddr.IP.String(), nil
	}
	return
}

/**
* @Description: 获取外网ip地址
* @return: string
* @Author: Iori
**/
func GetOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logger.Errorf("GetOutboundIP err | %v", err.Error())
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	fmt.Println(localAddr.String())
	return localAddr.IP.String()
}

/**
* @Description:  Strval 获取变量的字符串值,,浮点型 3.0将会转换成字符串3, "3",,非数值或字符类型的变量将会被转换成JSON格式字符串
* @param: value
* @return: string
**/
func StrVal(value interface{}) string {
	var key string
	if value == nil {
		return key
	}
	switch value.(type) {
	case float64:
		ft := value.(float64)
		key = strconv.FormatFloat(ft, 'f', -1, 64)
	case float32:
		ft := value.(float32)
		key = strconv.FormatFloat(float64(ft), 'f', -1, 64)
	case int:
		it := value.(int)
		key = strconv.Itoa(it)
	case uint:
		it := value.(uint)
		key = strconv.Itoa(int(it))
	case int8:
		it := value.(int8)
		key = strconv.Itoa(int(it))
	case uint8:
		it := value.(uint8)
		key = strconv.Itoa(int(it))
	case int16:
		it := value.(int16)
		key = strconv.Itoa(int(it))
	case uint16:
		it := value.(uint16)
		key = strconv.Itoa(int(it))
	case int32:
		it := value.(int32)
		key = strconv.Itoa(int(it))
	case uint32:
		it := value.(uint32)
		key = strconv.Itoa(int(it))
	case int64:
		it := value.(int64)
		key = strconv.FormatInt(it, 10)
	case uint64:
		it := value.(uint64)
		key = strconv.FormatUint(it, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	default:
		newValue, _ := json.Marshal(value)
		key = string(newValue)
	}
	return key
}

/**
* @Description: 是否為null值 ，default
* @param: i
* @return: bool
* @Author: Iori
**/
func IsNull(i interface{}) bool {
	value := reflect.ValueOf(i)

	switch value.Kind() {
	case reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return value.IsNil()
	}

	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

/**
* @Description: 三目运算之泛型
* @param: flag
* @param: a
* @param: b
* @return: R
* @Author: Iori
**/
func IfOr[R any](flag bool, a, b R) R {
	if flag {
		return a
	}
	return b
}

/**
* @Description:
* @param: hashmap
* @return: string
* @Author: Iori
**/
func MapToJson(hashmap interface{}) string {
	data, _ := json.Marshal(hashmap)
	return convert.Bytes2str(data)
}
