package http_sign

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/gofrs/uuid"
)

// ----------------------   const  --------------------
const (
	Const_SignVer_Name   string = "sign_ver"   //签名规范版本名称
	Const_SignType_Name  string = "sign_type"  //签名加密方式名称
	Const_TimeStamp_Name string = "timestamp"  //默认时间戳字段名
	Const_RequestId_Name string = "request_id" //默认时间戳字段名
	Const_SignName       string = "sign"       //默认签名字段名
	Const_ExpiredTime    int64  = 60           //默认签名有效时长
)

// ---------------------   enum ----------------------
// 版本类型
type EVersionType string

const (
	Version_NewV1 EVersionType = "1" //新签名规范版本1
	Version_NewV2 EVersionType = "2" //新签名规范版本2
)

var version_type = map[string]EVersionType{
	string(Version_NewV1): Version_NewV1,
	string(Version_NewV2): Version_NewV2,
}

// 签名类型
type ESignType string

const (
	Sign_Md5  ESignType = "md5"  //md5 加密方式
	Sign_Sha1 ESignType = "sha1" //sha1 加密方式
)

var sign_type = map[string]ESignType{
	string(Sign_Md5):  Sign_Md5,
	string(Sign_Sha1): Sign_Sha1,
}

// -- err code
type ErrorCode int32

const (
	SIGH_OK           ErrorCode = 0  //签名成功
	TIMESTAMP_INVALID ErrorCode = -1 //时间戳无效
	PARSE_FAIL        ErrorCode = -2 //解析失败
	TIME_OUT          ErrorCode = -3 //时间超时
	SIGN_NOT_FOUND    ErrorCode = -4 //签名未找到
	ARGUMENTS_INVALID ErrorCode = -5 //参数无效
	VERIFY_FAILURE    ErrorCode = -6 //签名验证失败
	SIGNTYPE_INVALID  ErrorCode = -7 //签名加密类型无效
)

var error_code_msg = map[int32]string{
	int32(SIGH_OK):           "SIGH_OK",
	int32(TIMESTAMP_INVALID): "TIMESTAMP_INVALID",
	int32(PARSE_FAIL):        "PARSE_FAIL",
	int32(TIME_OUT):          "TIMESTAMP_TIME_OUT",
	int32(SIGN_NOT_FOUND):    "SIGN_NOT_FOUND",
	int32(ARGUMENTS_INVALID): "ARGUMENTS_INVALID",
	int32(VERIFY_FAILURE):    "VERIFY_FAILURE",
	int32(SIGNTYPE_INVALID):  "SIGNTYPE_INVALID",
}

func (x ErrorCode) String() string {
	return error_code_msg[int32(x)]
}

// ----------------------  end

/*
*  HttpSign
*  @Description: 签名信息结构
*  @Author: Iori
 */
type HttpSign struct {
	//检验参数类型名，不会参与校验，如：sign|token|md5
	signName string
	//密钥
	secret string
	//有效期时长
	expiredTime int64
	//时间戳名
	timestampName string
	//请求唯一标识，如：uuid
	requestIdName string
	//版本类型
	versionType EVersionType
}

/**
* @Description: 创建httpSign对象
* @param: signName
* @param: secret
* @param: expiredTime
* @param: timestampName
* @param: requestIdName  请求唯一标识
* @return: *HttpSign
* @Author: Iori
* @Date: 2022-01-27 16:46:05
**/
func BuildHttpSign(signName, secret string, expiredTime int64, timestampName, requestIdName, version string) *HttpSign {
	//check args
	if "" == signName {
		signName = Const_SignName
	}
	if "" == timestampName {
		timestampName = Const_TimeStamp_Name
	}

	if expiredTime < 0 {
		expiredTime = Const_ExpiredTime
	}
	//new obj
	sign_ins := &HttpSign{}
	sign_ins.signName = signName
	sign_ins.secret = secret
	sign_ins.expiredTime = expiredTime
	sign_ins.requestIdName = requestIdName
	sign_ins.timestampName = timestampName
	sign_ins.versionType = sign_ins.toVersionType(version)

	return sign_ins
}

/**
* @Description: 时间戳检测
* @Author Iori
* @Date 2022-01-25 10:33:01
* @param signName, secret string, expiredTime int64, timestampName, signType
* @return  *HttpSign
**/
func (this *HttpSign) checkTimestamp(params *map[string]string) (bool, error, int) {
	//获取时间戳参数
	timestamp := (*params)[this.timestampName]
	//是否存在
	if timestamp == "" {
		return false, errors.New(TIMESTAMP_INVALID.String()), int(TIMESTAMP_INVALID)
	}

	reqTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false, err, int(PARSE_FAIL)
	}

	//时间范围检测,设置有效时间为0则跳过
	if this.expiredTime != 0 {
		cur_time := time.Now().Unix()
		if (cur_time - reqTime) > this.expiredTime {
			return false, errors.New(TIME_OUT.String()), int(TIME_OUT)
		}
	}

	return true, nil, int(SIGH_OK)
}

/**
 * @Description: 签名认证
 * @receiver this
 * @Date 2022-01-22 18:41:24
 * @param sign 待认证的签名
 * @return bool 认证成功返回tue 否则返回false
 * @return error 认证成功返回nil 否则返回异常值
 **/
func (this *HttpSign) CheckSign(head map[string][]string, params *map[string]string, body []byte, sign string) (bool, error, int) {
	if params == nil && body == nil && head == nil {
		return false, errors.New(ARGUMENTS_INVALID.String()), int(ARGUMENTS_INVALID)
	}

	//时间戳检测
	_time, _err, _code := this.checkTimestamp(params)
	if !_time {
		return false, _err, _code
	}

	//获取签名字段值
	if "" == sign {
		sign = (*params)[this.signName]
		if "" == sign {
			return false, errors.New(SIGN_NOT_FOUND.String()), int(SIGN_NOT_FOUND)
		}
	}

	//签名检测
	local_sign, uri_str := this.buildSign(head, params, body,
		this.toSignType((*params)[Const_SignType_Name]),
		this.toVersionType((*params)[Const_SignVer_Name]))
	if sign != local_sign {
		logger.Errorf("CheckSign -- error sign uriStr:| %v", uri_str)
		return false, errors.New(VERIFY_FAILURE.String()), int(VERIFY_FAILURE)
	}

	return true, nil, int(SIGH_OK)
}

/**
 * @Description: 添加签名
 * @receiver this
 * @Date 2022-01-22 18:41:24
 * @param sign 待认证的签名
 * @return bool 认证成功返回tue 否则返回false
 * @return error 认证成功返回nil 否则返回异常值
 **/
func (this *HttpSign) PushSign(head map[string][]string, params *map[string]string, body []byte,
	signType ESignType) *map[string]string {
	if params == nil {
		params = &map[string]string{}
	}

	//添加时间戳
	timestamp := (*params)[this.timestampName]
	if timestamp == "" {
		(*params)[this.timestampName] = strconv.Itoa(int(time.Now().Unix()))
	}

	//获取签名方式
	type_str := (*params)[Const_SignType_Name]
	_signType := this.toSignType(type_str)
	if _signType != signType && type_str != "" {
		signType = _signType
	}

	//requestid
	if "" != this.requestIdName {
		uuid, _ := uuid.NewV4()
		(*params)[this.requestIdName] = strings.ReplaceAll(uuid.String(), "-", "")
	}

	//签名type字段修正
	if signType == Sign_Md5 {
		delete((*params), Const_SignType_Name)
	} else {
		(*params)[Const_SignType_Name] = string(signType)
	}

	//添加签名与签名方式
	sign_, _ := this.buildSign(head, params, body, signType, this.versionType)
	(*params)[this.signName] = sign_
	return params
}

/**
 * @Description: 根据传入的signstr 取对应的ESignType类型
 * @Author Iori
 * @Date 2022-01-26 17:55:55
 * @param signType
 * @return ESignType
 **/
func (this *HttpSign) toSignType(signType string) ESignType {
	if _type, ok := sign_type[strings.ToLower(signType)]; ok {
		return _type
	}

	//default
	return Sign_Md5
}

/**
 * @Description: 根据传入的str 取对应的EVersionType类型
 * @Author Iori
 * @Date 2022-03-08 17:55:55
 * @param verStr
 * @return EVersionType
 **/
func (this *HttpSign) toVersionType(verStr string) EVersionType {
	if verType, ok := version_type[strings.ToLower(verStr)]; ok {
		return verType
	}

	//default
	return Version_NewV1
}

/**
 * @Description: 通过uri的参数列表构建sign
 * @Author Iori
 * @Date 2022-01-22 17:55:55
 * @param params 参数列表map，其中k为参数名，v为参数值
 * @param body http请求中以post等协议携带的包体中的内容(可为nil)
 * @param secret 盐值
 * @return string 签名值
 **/
func (this *HttpSign) buildSign(head map[string][]string, params *map[string]string, body []byte,
	signType ESignType, verType EVersionType) (string, string) {
	var uriStr bytes.Buffer

	if params == nil {
		params = &map[string]string{}
	}

	uriStr.WriteString(Map2uri(params, this.signName, true, false))
	uriStr.Write(body)
	uriStr.WriteString(this.secret)

	//crypto
	sign := ""
	switch signType {
	case Sign_Sha1:
		sign = fmt.Sprintf("%x", sha1.Sum(uriStr.Bytes()))
	case Sign_Md5:
		sign = fmt.Sprintf("%x", md5.Sum(uriStr.Bytes()))
	default: //md5
		sign = fmt.Sprintf("%x", md5.Sum(uriStr.Bytes()))
	}

	return sign, uriStr.String()
}
