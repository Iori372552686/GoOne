// Package http_sign 计算并校验 HTTP 请求签名。
//
// 支持两类算法，可通过 Config.SignType 为每个 HttpSign 实例单独选择：
//
//   - "md5" / "sha1"    ：旧式明文哈希，签名内容为 "<params>&<body><secret>"。
//     为向后兼容保留，签名输出与原实现逐字节一致，已部署客户端无感。
//   - "hmac_sha256"     ：以 secret 为密钥对 "<params>&<body>" 做 HMAC-SHA256。
//     新部署推荐使用。
//
// 无论使用哪种算法，签名比较一律采用 hmac.Equal（恒定时间比较）。
package http_sign

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/gofrs/uuid"
)

// 默认字段名与默认签名有效期。
const (
	Const_SignVer_Name   = "sign_ver"   // 签名规范版本字段名
	Const_SignType_Name  = "sign_type"  // 签名算法字段名
	Const_TimeStamp_Name = "timestamp"  // 默认时间戳字段名
	Const_RequestId_Name = "request_id" // 默认请求唯一标识字段名
	Const_SignName       = "sign"       // 默认签名字段名
	Const_ExpiredTime    = 60           // 默认签名有效期（秒）
)

// EVersionType 标识签名规范版本。
//
// Deprecated：版本分派仅为兼容保留在公开接口上，当前并无实际行为差异。
type EVersionType string

const (
	Version_NewV1 EVersionType = "1"
	Version_NewV2 EVersionType = "2"
)

var versionType = map[string]EVersionType{
	string(Version_NewV1): Version_NewV1,
	string(Version_NewV2): Version_NewV2,
}

// ESignType 标识签名算法。
type ESignType string

const (
	Sign_Md5        ESignType = "md5"         // 旧式 MD5
	Sign_Sha1       ESignType = "sha1"        // 旧式 SHA-1
	Sign_HMacSha256 ESignType = "hmac_sha256" // HMAC-SHA256（推荐）
)

var signType = map[string]ESignType{
	string(Sign_Md5):        Sign_Md5,
	string(Sign_Sha1):       Sign_Sha1,
	string(Sign_HMacSha256): Sign_HMacSha256,
}

// ErrorCode 表示一次签名校验的结果码。
type ErrorCode int32

const (
	SignOK           ErrorCode = 0  // 签名校验通过
	ErrTimestamp     ErrorCode = -1 // 时间戳字段缺失
	ErrParse         ErrorCode = -2 // 时间戳解析失败
	ErrTimeout       ErrorCode = -3 // 时间戳超出有效期
	ErrSignNotFound  ErrorCode = -4 // 签名字段缺失
	ErrArguments     ErrorCode = -5 // 入参非法
	ErrVerifyFailure ErrorCode = -6 // 签名比对不一致
)

var errorCodeMsg = map[ErrorCode]string{
	SignOK:           "SIGH_OK",
	ErrTimestamp:     "TIMESTAMP_INVALID",
	ErrParse:         "PARSE_FAIL",
	ErrTimeout:       "TIMESTAMP_TIME_OUT",
	ErrSignNotFound:  "SIGN_NOT_FOUND",
	ErrArguments:     "ARGUMENTS_INVALID",
	ErrVerifyFailure: "VERIFY_FAILURE",
}

func (c ErrorCode) String() string { return errorCodeMsg[c] }

// HttpSign 负责针对单个 secret 计算与校验请求签名。
type HttpSign struct {
	signName      string // 签名字段名（不参与签名内容）
	secret        string // 共享密钥 / HMAC 密钥
	expiredTime   int64  // 有效期（秒）；0 表示不校验
	timestampName string // 时间戳字段名
	requestIdName string // 请求唯一标识字段名（如 uuid）；空串表示不启用
	versionType   EVersionType
	defaultSign   ESignType // 未在报文中显式指定算法时使用的默认算法
}

// BuildHttpSign 构造一个 HttpSign 实例，对空串或非法字段套用默认值。
// expiredTime 为 0 表示关闭时间戳窗口校验；负数回退到 Const_ExpiredTime。
func BuildHttpSign(signName, secret string, expiredTime int64, timestampName, requestIdName, version string) *HttpSign {
	s := &HttpSign{
		signName:      signName,
		secret:        secret,
		expiredTime:   expiredTime,
		timestampName: timestampName,
		requestIdName: requestIdName,
		versionType:   toVersionType(version),
		defaultSign:   Sign_Md5,
	}
	if signName == "" {
		s.signName = Const_SignName
	}
	if timestampName == "" {
		s.timestampName = Const_TimeStamp_Name
	}
	if expiredTime < 0 {
		s.expiredTime = int64(Const_ExpiredTime)
	}
	return s
}

// WithSignType 设置 PushSign 在调用方未显式指定算法时使用的默认算法。
// 通过该接口可实现“改配置不改调用代码”的算法切换。返回接收者以便链式调用。
func (s *HttpSign) WithSignType(t ESignType) *HttpSign {
	s.defaultSign = t
	return s
}

// SignType 返回本实例默认使用的签名算法。
func (s *HttpSign) SignType() ESignType { return s.defaultSign }

// CheckSign 校验请求签名。params 为查询参数，body 为请求体，sign 为可选的
// 显式签名串（为空时从 params[signName] 读取）。比较采用恒定时间。
//
// 校验通过返回 (SignOK, nil)，否则返回对应的 ErrorCode 与描述错误。
func (s *HttpSign) CheckSign(params map[string]string, body []byte, sign string) (ErrorCode, error) {
	if params == nil && body == nil {
		return ErrArguments, errors.New(ErrArguments.String())
	}

	// 时间戳新鲜度校验（双边，拒绝远未来时间戳）
	if code, err := s.checkTimestamp(params); code != SignOK {
		return code, err
	}

	// 解析待校验签名
	if sign == "" {
		if sign = params[s.signName]; sign == "" {
			return ErrSignNotFound, errors.New(ErrSignNotFound.String())
		}
	}

	// 重建签名并恒定时间比较
	local, debugURI := s.buildSign(params, body, toSignType(params[Const_SignType_Name]), s.versionType)
	if !hmac.Equal([]byte(sign), []byte(local)) {
		logger.Errorf("CheckSign -- 签名不一致, 期望签名内容 uriStr: %s", debugURI)
		return ErrVerifyFailure, errors.New(ErrVerifyFailure.String())
	}
	return SignOK, nil
}

// PushSign 向 params 注入时间戳、请求唯一标识与签名（就地修改并返回），
// 用于对外发起的已签名请求。params 为 nil 时会新建 map。
//
// 签名算法由实例默认算法（见 WithSignType / SignType）决定；若 params
// 中已带 sign_type 字段则以该字段为准。md5 会从报文中剔除 sign_type 字段，
// 以保持与旧客户端的逐字节兼容。
//
// 算法不作为参数暴露，避免调用方与实例配置出现两套真实来源。
func (s *HttpSign) PushSign(params map[string]string, body []byte) map[string]string {
	if params == nil {
		params = make(map[string]string)
	}

	// 时间戳：已有则保留，否则盖上当前时间
	if params[s.timestampName] == "" {
		params[s.timestampName] = strconv.FormatInt(time.Now().Unix(), 10)
	}

	// 算法解析：报文显式值优先，否则用实例默认值。
	signType := s.defaultSign
	if wire := params[Const_SignType_Name]; wire != "" {
		signType = toSignType(wire)
	}

	// 可选的请求唯一标识（如 uuid，去掉中划线）
	if s.requestIdName != "" {
		if u, err := uuid.NewV4(); err == nil {
			params[s.requestIdName] = strings.ReplaceAll(u.String(), "-", "")
		}
	}

	// 规整 sign_type 字段：md5 剔除该字段（保持旧报文格式），
	// 其余算法写入算法名。
	if signType == Sign_Md5 {
		delete(params, Const_SignType_Name)
	} else {
		params[Const_SignType_Name] = string(signType)
	}

	params[s.signName], _ = s.buildSign(params, body, signType, s.versionType)
	return params
}

// checkTimestamp 按配置的有效期做对称（双边）校验。
// expiredTime 为 0 表示关闭校验。时间戳缺失或无法解析会单独上报错误码。
func (s *HttpSign) checkTimestamp(params map[string]string) (ErrorCode, error) {
	timestamp := params[s.timestampName]
	if timestamp == "" {
		return ErrTimestamp, errors.New(ErrTimestamp.String())
	}

	reqTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrParse, err
	}

	if s.expiredTime == 0 {
		return SignOK, nil
	}

	// 对称偏差校验：既拒绝过期时间戳，也拒绝远未来时间戳
	if skew := time.Now().Unix() - reqTime; skew > s.expiredTime || skew < -s.expiredTime {
		return ErrTimeout, errors.New(ErrTimeout.String())
	}
	return SignOK, nil
}

// buildSign 计算给定 params+body 的签名，并返回一份不含 secret 的签名内容
// 供校验失败时打印调试日志（有意不输出 secret，避免经日志泄露密钥）。
//
// md5/sha1 的签名内容为 "params+body+secret"（与原实现逐字节一致）；
// hmac_sha256 以 secret 为密钥对 "params+body" 做 HMAC。
func (s *HttpSign) buildSign(params map[string]string, body []byte, signType ESignType, _ EVersionType) (sign, debugURI string) {
	// 统一构建不含 secret 的签名内容（同时也是 debugURI），供各算法复用。
	paramsStr := Map2uri(params, s.signName, true, false)
	debugURI = paramsStr + string(body)

	switch signType {
	case Sign_HMacSha256:
		// HMAC：secret 作为密钥，params+body 作为消息
		mac := hmac.New(sha256.New, []byte(s.secret))
		mac.Write([]byte(paramsStr))
		mac.Write(body)
		return hex.EncodeToString(mac.Sum(nil)), debugURI

	default:
		// md5 / sha1（及任何未知类型）：旧式布局 params+body+secret，
		// 输出与原实现逐字节一致。复用 pool buffer 拼接以减少分配。
		buf := getBuffer()
		defer putBuffer(buf)
		buf.WriteString(paramsStr)
		buf.Write(body)
		buf.WriteString(s.secret)

		payload := buf.Bytes()
		if signType == Sign_Sha1 {
			sum := sha1.Sum(payload)
			return hex.EncodeToString(sum[:]), debugURI
		}
		sum := md5.Sum(payload)
		return hex.EncodeToString(sum[:]), debugURI
	}
}

// toSignType 解析算法名，未知名回退为 md5。
func toSignType(name string) ESignType {
	if t, ok := signType[strings.ToLower(name)]; ok {
		return t
	}
	return Sign_Md5
}

// toVersionType 解析版本名，未知名回退为 V1。
func toVersionType(name string) EVersionType {
	if v, ok := versionType[strings.ToLower(name)]; ok {
		return v
	}
	return Version_NewV1
}
