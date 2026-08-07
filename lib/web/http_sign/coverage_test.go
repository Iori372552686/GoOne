package http_sign

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// 本文件用于补全 coverage_test 中遗漏的功能分支与边界场景，
// 与 sign_test.go 互补，共同把单测覆盖率推到 95% 以上。

// TestBuildHttpSign_Defaults 覆盖构造函数的默认值回退分支：
// 空 signName/timestampName 套默认值，负 expiredTime 回退到默认。
func TestBuildHttpSign_Defaults(t *testing.T) {
	s := BuildHttpSign("", "secret", -1, "", "", "")
	if s.signName != Const_SignName {
		t.Fatalf("空 signName 应回退为默认: got %q", s.signName)
	}
	if s.timestampName != Const_TimeStamp_Name {
		t.Fatalf("空 timestampName 应回退为默认: got %q", s.timestampName)
	}
	if s.expiredTime != int64(Const_ExpiredTime) {
		t.Fatalf("负 expiredTime 应回退为默认: got %d", s.expiredTime)
	}
	if s.defaultSign != Sign_Md5 {
		t.Fatalf("默认算法应为 md5: got %s", s.defaultSign)
	}

	// expiredTime=0 是合法值（关闭校验），不应被改写
	s0 := BuildHttpSign("sign", "secret", 0, "timestamp", "", "")
	if s0.expiredTime != 0 {
		t.Fatalf("expiredTime=0 应保留: got %d", s0.expiredTime)
	}
}

// TestSignType_Getter 覆盖 SignType() getter 与 WithSignType 链式调用。
func TestSignType_Getter(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
	if got := s.SignType(); got != Sign_Md5 {
		t.Fatalf("默认 SignType 应为 md5: got %s", got)
	}
	ret := s.WithSignType(Sign_HMacSha256)
	if ret != s {
		t.Fatalf("WithSignType 应返回接收者本身以便链式调用")
	}
	if got := s.SignType(); got != Sign_HMacSha256 {
		t.Fatalf("设置后 SignType 应为 hmac: got %s", got)
	}
}

// TestMapParam2Uri 覆盖公开封装 MapParam2Uri 的排序关闭与 URL 编码路径。
func TestMapParam2Uri(t *testing.T) {
	// nil 入参应返回空串
	if got := MapParam2Uri(nil, false); got != "" {
		t.Fatalf("nil 入参应返回空串: got %q", got)
	}

	// 不编码
	params := map[string]string{"b": "2", "a": "1"}
	if got := MapParam2Uri(params, false); got != "b=2&a=1" {
		t.Fatalf("不排序不编码应原样输出: got %q", got)
	}

	// 带 URL 编码：空格应被转成 +
	enc := map[string]string{"q": "hello world", "x": "a&b"}
	got := MapParam2Uri(enc, true)
	if !strings.Contains(got, "q=hello+world") {
		t.Fatalf("URL 编码应转空格为 +: got %q", got)
	}
	if !strings.Contains(got, "x=a%26b") {
		t.Fatalf("URL 编码应转 & 为 %%26: got %q", got)
	}
}

// TestSignMgr_InitAndRun 覆盖从 Config 列表批量构建实例的入口，
// 验证 Config.SignType 能正确驱动 WithSignType。
func TestSignMgr_InitAndRun(t *testing.T) {
	m := NewSignMgr()
	cfgs := []Config{
		{IndexName: "default", PrivateKey: "k1", SignName: "", ExpiredTime: 60, TimestampName: "", SignType: "md5"},
		{IndexName: "hmac", PrivateKey: "k2", SignName: "sign", ExpiredTime: 30, TimestampName: "ts", SignType: "hmac_sha256"},
		{IndexName: "sha1", PrivateKey: "k3", SignName: "sign", ExpiredTime: 0, TimestampName: "ts", SignType: "sha1"},
	}
	m.InitAndRun(cfgs)

	if m.Count() != 3 {
		t.Fatalf("应注册 3 个实例: got %d", m.Count())
	}
	// default 实例：空 SignName/TimestampName 套默认，算法 md5
	def := m.GetSignIns()
	if def == nil {
		t.Fatalf("default 实例应存在")
	}
	if def.SignType() != Sign_Md5 {
		t.Fatalf("default 算法应为 md5: got %s", def.SignType())
	}
	// hmac 实例：算法由配置驱动
	hm := m.GetSignIns("hmac")
	if hm == nil || hm.SignType() != Sign_HMacSha256 {
		t.Fatalf("hmac 实例算法应为 hmac_sha256")
	}
	// sha1 实例
	sh := m.GetSignIns("sha1")
	if sh == nil || sh.SignType() != Sign_Sha1 {
		t.Fatalf("sha1 实例算法应为 sha1")
	}
	// 不存在的 key 返回 nil
	if m.GetSignIns("nope") != nil {
		t.Fatalf("不存在的 key 应返回 nil")
	}
}

// TestSignMgr_Count_NilReceiver 覆盖 Count 在 nil 接收者上的安全行为。
func TestSignMgr_Count_NilReceiver(t *testing.T) {
	var nilMgr *SignMgr
	if got := nilMgr.Count(); got != 0 {
		t.Fatalf("nil SignMgr.Count 应返回 0: got %d", got)
	}
}

// TestCheckSign_NilArgs 覆盖 params 与 body 同时为 nil 的 ErrArguments 分支。
func TestCheckSign_NilArgs(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
	code, err := s.CheckSign(nil, nil, "")
	if code != ErrArguments {
		t.Fatalf("params+body 全 nil 应返回 ErrArguments: got %s err=%v", code, err)
	}
}

// TestCheckSign_TimestampParseFail 覆盖时间戳非数字时的 ErrParse 分支。
func TestCheckSign_TimestampParseFail(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 60, "timestamp", "", "1")
	params := map[string]string{"timestamp": "not-a-number"}
	code, err := s.CheckSign(params, []byte("body"), "xxxx")
	if code != ErrParse {
		t.Fatalf("时间戳非数字应返回 ErrParse: got %s err=%v", code, err)
	}
}

// TestCheckSign_TimestampMissing 覆盖缺少时间戳字段分支。
func TestCheckSign_TimestampMissing(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 60, "timestamp", "", "1")
	params := map[string]string{}
	code, _ := s.CheckSign(params, []byte("body"), "x")
	if code != ErrTimestamp {
		t.Fatalf("缺少 timestamp 应返回 ErrTimestamp: got %s", code)
	}
}

// TestPushSign_RequestIdInjection 覆盖 requestIdName 非空时注入 uuid 的路径。
func TestPushSign_RequestIdInjection(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "request_id", "1")
	params := map[string]string{}
	s.PushSign(params, []byte("body"))
	rid := params["request_id"]
	if rid == "" {
		t.Fatalf("requestIdName 非空时应注入 request_id")
	}
	if strings.Contains(rid, "-") {
		t.Fatalf("request_id 应去掉中划线: got %q", rid)
	}
	// 两次注入应得到不同值
	params2 := map[string]string{}
	s.PushSign(params2, []byte("body"))
	if params2["request_id"] == rid {
		t.Fatalf("两次注入的 request_id 不应相同")
	}
}

// TestPushSign_NilParams 覆盖 params 为 nil 时自动建 map 的路径。
func TestPushSign_NilParams(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
	out := s.PushSign(nil, []byte("body"))
	if out == nil {
		t.Fatalf("nil params 应返回新建的 map")
	}
	if out["sign"] == "" {
		t.Fatalf("应已写入签名字段")
	}
}

// TestUriParam2Map_EdgeCases 覆盖空串、无 = 的 pair、空 pair。
func TestUriParam2Map_EdgeCases(t *testing.T) {
	// 空串
	m := UriParam2Map("")
	if len(m) != 0 {
		t.Fatalf("空串应得到空 map: got %v", m)
	}
	// 含无 = 的 pair 与空 pair，应被跳过
	m = UriParam2Map("a=1&invalid&b=2&&c=")
	if m["a"] != "1" || m["b"] != "2" {
		t.Fatalf("应保留有效项: got %v", m)
	}
	if _, ok := m["invalid"]; ok {
		t.Fatalf("无 = 的 pair 应被跳过")
	}
	if _, ok := m["c"]; !ok {
		t.Fatalf("c= 的空值项应保留 key")
	}
}

// TestMap2uri_URLEncode 覆盖 Map2uri 的 encode=true 分支。
func TestMap2uri_URLEncode(t *testing.T) {
	params := map[string]string{"a": "1", "q": "x y"}
	got := Map2uri(params, "", true, true)
	if !strings.Contains(got, "q=x+y") {
		t.Fatalf("encode=true 应转空格: got %q", got)
	}
}

// TestMap2uri_NoFilter 覆盖 filter 为空、不剔除任何字段。
func TestMap2uri_NoFilter(t *testing.T) {
	params := map[string]string{"sign": "abc", "a": "1"}
	got := Map2uri(params, "", true, false)
	if !strings.Contains(got, "sign=abc") {
		t.Fatalf("空 filter 不应剔除 sign 字段: got %q", got)
	}
}

// TestPutBuffer_Oversized 覆盖 pool.putBuffer 对超大 buffer 不回收的分支。
// 触发方式：构造一个 cap > 64KB 的 buffer 归还，再 Get 一个新 buffer，
// 验证其并非刚归还的那个（容量应回到默认 256）。
func TestPutBuffer_Oversized(t *testing.T) {
	big := &poolBuffer{buf: make([]byte, 0, 70*1024)} // > 64KB
	putBuffer(big) // 应不回收

	got := getBuffer()
	if cap(got.buf) >= 70*1024 {
		t.Fatalf("超大 buffer 不应被回收复用: cap=%d", cap(got.buf))
	}
	putBuffer(got)
}

// TestErrorCode_String 覆盖所有 ErrorCode 的 String() 输出。
func TestErrorCode_String(t *testing.T) {
	cases := []struct {
		code ErrorCode
		msg  string
	}{
		{SignOK, "SIGH_OK"},
		{ErrTimestamp, "TIMESTAMP_INVALID"},
		{ErrParse, "PARSE_FAIL"},
		{ErrTimeout, "TIMESTAMP_TIME_OUT"},
		{ErrSignNotFound, "SIGN_NOT_FOUND"},
		{ErrArguments, "ARGUMENTS_INVALID"},
		{ErrVerifyFailure, "VERIFY_FAILURE"},
	}
	for _, c := range cases {
		if got := c.code.String(); got != c.msg {
			t.Fatalf("ErrorCode(%d).String() = %q, want %q", c.code, got, c.msg)
		}
	}
}

// TestToVersionType 覆盖 toVersionType 的 V2 命中与未知回退分支。
func TestToVersionType(t *testing.T) {
	cases := []struct {
		in   string
		want EVersionType
	}{
		{"1", Version_NewV1},
		{"2", Version_NewV2},
		{"9", Version_NewV1}, // 未知回退 V1
		{"", Version_NewV1},
	}
	for _, c := range cases {
		if got := toVersionType(c.in); got != c.want {
			t.Fatalf("toVersionType(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestCheckSign_SHA1_RoundTrip 端到端覆盖 SHA1 算法的 PushSign→CheckSign 往返。
func TestCheckSign_SHA1_RoundTrip(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1").WithSignType(Sign_Sha1)
	params := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix(), 10)}
	body := []byte(`{"x":1}`)
	s.PushSign(params, body)
	if code, _ := s.CheckSign(params, body, ""); code != SignOK {
		t.Fatalf("sha1 往返应通过: got %s", code)
	}
	// 报文应显式带 sign_type=sha1（非 md5 时需写入）
	if params["sign_type"] != "sha1" {
		t.Fatalf("非 md5 算法应写入 sign_type 字段: got %q", params["sign_type"])
	}
}

// TestCheckSign_BodyOnly_ParamsNil 覆盖 params 为 nil、body 非 nil 的场景。
// 该场景下读 nil map 不 panic，但时间戳缺失应返回 ErrTimestamp。
func TestCheckSign_BodyOnly_ParamsNil(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 60, "timestamp", "", "1")
	code, _ := s.CheckSign(nil, []byte("body"), "x")
	if code != ErrTimestamp {
		t.Fatalf("params 为 nil 时应因时间戳缺失返回 ErrTimestamp: got %s", code)
	}
}
