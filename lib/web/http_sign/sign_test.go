package http_sign

import (
	"strconv"
	"testing"
	"time"
)

// 这些黄金值由原实现（params+body+secret 的 MD5/SHA1，以及
// params+body 的 HMAC-SHA256）离线计算得出，用于锁定签名输出的字节级兼容性。
// 一旦 MD5/SHA1 分支的拼接顺序或摘要方式被改动，此处会立即失败。
const (
	md5Golden  = "484ea76937e5a7e1bddf7dec4362cd22"
	sha1Golden = "ea865e15054b97195abdfd5afbdcc3d7e14b339b"
	hmacGolden = "571253c059913ca7a13db5d299f8f49da1a97303cb15a676448ea6988469bc17"
	md5Golden2 = "346cb40326187b4fd5cd3eedbda86e52"
)

// 构造一组固定的测试参数与请求体（与生成黄金值时的输入完全一致）。
func signFixture() (map[string]string, []byte) {
	return map[string]string{
			"timestamp": "1700000000",
			"user":      "alice",
			"action":    "login",
		}, []byte(`{"hello":"world"}`)
}

// TestBuildSign_MD5_ByteCompatible 锁定 MD5 签名输出与原实现完全一致。
func TestBuildSign_MD5_ByteCompatible(t *testing.T) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params, body := signFixture()

	sign, _ := s.buildSign(params, body, Sign_Md5, Version_NewV1)
	if sign != md5Golden {
		t.Fatalf("md5 签名与黄金值不一致: got %s want %s", sign, md5Golden)
	}
}

// TestBuildSign_SHA1_ByteCompatible 锁定 SHA1 签名输出与原实现完全一致。
func TestBuildSign_SHA1_ByteCompatible(t *testing.T) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params, body := signFixture()

	sign, _ := s.buildSign(params, body, Sign_Sha1, Version_NewV1)
	if sign != sha1Golden {
		t.Fatalf("sha1 签名与黄金值不一致: got %s want %s", sign, sha1Golden)
	}
}

// TestBuildSign_HMACSHA256 校验 HMAC-SHA256 新算法的正确性。
func TestBuildSign_HMACSHA256(t *testing.T) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params, body := signFixture()

	sign, _ := s.buildSign(params, body, Sign_HMacSha256, Version_NewV1)
	if sign != hmacGolden {
		t.Fatalf("hmac 签名与黄金值不一致: got %s want %s", sign, hmacGolden)
	}
}

// TestBuildSign_MD5_NoBody 校验无 body 场景下的 MD5 输出。
func TestBuildSign_MD5_NoBody(t *testing.T) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params := map[string]string{"a": "1", "b": "2"}

	sign, _ := s.buildSign(params, nil, Sign_Md5, Version_NewV1)
	if sign != md5Golden2 {
		t.Fatalf("无 body 的 md5 签名与黄金值不一致: got %s want %s", sign, md5Golden2)
	}
}

// TestCheckSign_RoundTrip 验证 PushSign 后 CheckSign 能通过，且实例默认算法切换生效。
func TestCheckSign_RoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name     string
		signType ESignType
	}{
		{"md5", Sign_Md5},
		{"sha1", Sign_Sha1},
		{"hmac", Sign_HMacSha256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1").
				WithSignType(tc.signType)
			params, body := signFixture()
			// 清掉 fixture 里的固定时间戳，让 PushSign 盖当前时间
			delete(params, "timestamp")

			s.PushSign(params, body)
			if code, err := s.CheckSign(params, body, ""); code != SignOK || err != nil {
				t.Fatalf("%s 往返校验失败: code=%s err=%v", tc.name, code, err)
			}
		})
	}
}

func TestCheckSignRejectsAlgorithmDowngrade(t *testing.T) {
	server := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1").
		WithSignType(Sign_HMacSha256)
	body := []byte(`{"hello":"world"}`)

	tests := []struct {
		name     string
		wireType string
	}{
		{name: "missing sign_type"},
		{name: "explicit md5", wireType: string(Sign_Md5)},
		{name: "unknown type", wireType: "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix(), 10)}
			if tc.wireType != "" {
				params[Const_SignType_Name] = tc.wireType
			}
			legacy := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
			params["sign"], _ = legacy.buildSign(params, body, Sign_Md5, Version_NewV1)

			if code, err := server.CheckSign(params, body, ""); err == nil || code != ErrSignType {
				t.Fatalf("downgrade accepted: code=%v err=%v", code, err)
			}
		})
	}
}

func TestCheckSignConfiguredAlgorithmCompatibility(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	hmacSigner := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1").
		WithSignType(Sign_HMacSha256)
	hmacParams := hmacSigner.PushSign(map[string]string{}, body)
	if code, err := hmacSigner.CheckSign(hmacParams, body, ""); err != nil || code != SignOK {
		t.Fatalf("configured HMAC rejected: code=%v err=%v", code, err)
	}

	legacyMD5 := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	legacyParams := legacyMD5.PushSign(map[string]string{}, body)
	if _, exists := legacyParams[Const_SignType_Name]; exists {
		t.Fatal("legacy MD5 wire format must omit sign_type")
	}
	if code, err := legacyMD5.CheckSign(legacyParams, body, ""); err != nil || code != SignOK {
		t.Fatalf("legacy MD5 rejected: code=%v err=%v", code, err)
	}
}

// TestPushSign_WireTypePriority 验证报文 sign_type 字段优先级高于实例默认：
// 即便实例默认是 md5，报文显式带 hmac_sha256 时应以 hmac 签名，
// 反之实例默认 hmac 时报文不带字段则以 md5 兼容旧客户端。
func TestPushSign_WireTypePriority(t *testing.T) {
	params, body := signFixture()
	delete(params, "timestamp")

	// 实例默认 md5，报文显式 hmac -> 应使用 hmac
	sMd5 := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1").WithSignType(Sign_Md5)
	p1 := map[string]string{"timestamp": "1700000000", "sign_type": "hmac_sha256"}
	sMd5.PushSign(p1, body)
	expectedHMAC, _ := sMd5.buildSign(p1, body, Sign_HMacSha256, Version_NewV1)
	if p1["sign"] != expectedHMAC {
		t.Fatalf("报文显式 hmac 应优先生效: got %s want %s", p1["sign"], expectedHMAC)
	}

	// 实例默认 hmac，报文无 sign_type -> 应使用 hmac 并写入字段
	sHMAC := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1").WithSignType(Sign_HMacSha256)
	p2 := map[string]string{"timestamp": "1700000000"}
	sHMAC.PushSign(p2, body)
	if p2["sign_type"] != "hmac_sha256" {
		t.Fatalf("非 md5 默认应写入 sign_type 字段: got %q", p2["sign_type"])
	}

	// 实例默认 md5，报文无 sign_type -> 应剔除 sign_type 字段（旧报文兼容）
	p3 := map[string]string{"timestamp": "1700000000"}
	sMd5.PushSign(p3, body)
	if _, exists := p3["sign_type"]; exists {
		t.Fatalf("md5 默认应剔除 sign_type 字段")
	}
}

// TestCheckSign_TamperedBody 篡改请求体后签名应校验失败。
func TestCheckSign_TamperedBody(t *testing.T) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params := map[string]string{}
	body := []byte(`{"hello":"world"}`)

	s.PushSign(params, body)
	code, err := s.CheckSign(params, []byte(`{"hello":"tampered"}`), "")
	if code != ErrVerifyFailure {
		t.Fatalf("篡改 body 后应校验失败, got code=%s err=%v", code, err)
	}
}

// TestCheckSign_TimestampBothSides 验证时间戳双边校验：
// 过期与远未来均应被拒绝（修复原实现仅单边检测的缺陷）。
func TestCheckSign_TimestampBothSides(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 60, "timestamp", "", "1")

	// 远未来时间戳应被拒绝（原实现会错误放行）
	params := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix()+3600, 10)}
	if code, _ := s.checkTimestamp(params); code != ErrTimeout {
		t.Fatalf("远未来时间戳应被拒绝, got code=%s", code)
	}

	// 过期时间戳应被拒绝
	params["timestamp"] = strconv.FormatInt(time.Now().Unix()-3600, 10)
	if code, _ := s.checkTimestamp(params); code != ErrTimeout {
		t.Fatalf("过期时间戳应被拒绝, got code=%s", code)
	}

	// 当前时间戳应通过
	params["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	if code, _ := s.checkTimestamp(params); code != SignOK {
		t.Fatalf("当前时间戳应通过, got code=%s", code)
	}
}

// TestCheckSign_DisabledExpiry 验证 expiredTime=0 时关闭时间戳校验。
func TestCheckSign_DisabledExpiry(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
	params := map[string]string{"timestamp": "1"} // 远古时间戳
	if code, _ := s.checkTimestamp(params); code != SignOK {
		t.Fatalf("expiredTime=0 应关闭校验, got code=%s", code)
	}
}

// TestCheckSign_MissingSign 缺少签名字段时返回对应错误码。
func TestCheckSign_MissingSign(t *testing.T) {
	s := BuildHttpSign("sign", "secret", 0, "timestamp", "", "1")
	params := map[string]string{"timestamp": strconv.FormatInt(time.Now().Unix(), 10)}
	if code, _ := s.CheckSign(params, nil, ""); code != ErrSignNotFound {
		t.Fatalf("缺少签名字段应返回 ErrSignNotFound, got code=%s", code)
	}
}

// TestUriParam2Map_ValueWithEquals 值中含 '=' 不应被截断（修复原 Split 的缺陷）。
func TestUriParam2Map_ValueWithEquals(t *testing.T) {
	m := UriParam2Map("a=1&url=http://x?a=1&b=2")
	if m["a"] != "1" {
		t.Fatalf("a 解析错误: got %q", m["a"])
	}
	if m["url"] != "http://x?a=1" {
		t.Fatalf("含 = 的值被截断: got %q", m["url"])
	}
	if m["b"] != "2" {
		t.Fatalf("b 解析错误: got %q", m["b"])
	}
}

// TestMap2uri_SortedAndFilter 验证排序、字段过滤、空值跳过行为。
func TestMap2uri_SortedAndFilter(t *testing.T) {
	params := map[string]string{
		"sign":  "should-be-filtered",
		"b":     "2",
		"a":     "1",
		"empty": "",
	}
	got := Map2uri(params, "sign", true, false)
	// 期望：按字典序，sign 被过滤，空值被跳过
	want := "a=1&b=2"
	if got != want {
		t.Fatalf("Map2uri 输出不符: got %q want %q", got, want)
	}
}

// TestMap2uri_Empty 空或 nil 入参应返回空串。
func TestMap2uri_Empty(t *testing.T) {
	if got := Map2uri(nil, "", true, false); got != "" {
		t.Fatalf("nil 入参应返回空串, got %q", got)
	}
	if got := Map2uri(map[string]string{}, "", true, false); got != "" {
		t.Fatalf("空 map 入参应返回空串, got %q", got)
	}
}

// TestMap2uri_FirstValueEmpty 第一个值为空被跳过时，后续项不应误加前导分隔符。
// （回归保护：原实现以输出长度判断分隔符，重构时若误用索引会出错）
func TestMap2uri_FirstValueEmpty(t *testing.T) {
	params := map[string]string{
		"a": "",    // 排序后第一个，被跳过
		"b": "2",
	}
	got := Map2uri(params, "", true, false)
	if got != "b=2" {
		t.Fatalf("首项为空时不应有多余分隔符: got %q", got)
	}
}

// TestToSignType 验证算法名解析与默认回退。
func TestToSignType(t *testing.T) {
	cases := []struct {
		in   string
		want ESignType
	}{
		{"md5", Sign_Md5},
		{"MD5", Sign_Md5},
		{"sha1", Sign_Sha1},
		{"hmac_sha256", Sign_HMacSha256},
		{"HMAC_SHA256", Sign_HMacSha256},
		{"unknown", Sign_Md5}, // 未知回退 md5
		{"", Sign_Md5},
	}
	for _, c := range cases {
		if got := toSignType(c.in); got != c.want {
			t.Fatalf("toSignType(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestSignMgr_Registry 验证 SignMgr 的注册、默认 key 与按名获取。
func TestSignMgr_Registry(t *testing.T) {
	m := NewSignMgr()
	if ins := m.GetSignIns(); ins != nil {
		t.Fatalf("空 mgr 默认 key 应返回 nil")
	}

	def := BuildHttpSign("sign", "secret-default", 0, "timestamp", "", "1")
	other := BuildHttpSign("sign", "secret-other", 0, "timestamp", "", "1")
	m.SetSignIns("default", def)
	m.SetSignIns("other", other)

	if m.GetSignIns() != def {
		t.Fatalf("默认 key 取回不符")
	}
	if m.GetSignIns("other") != other {
		t.Fatalf("按名取回不符")
	}
	if m.Count() != 2 {
		t.Fatalf("Count 应为 2, got %d", m.Count())
	}
}

// BenchmarkBuildSign 度量签名计算热路径性能。
func BenchmarkBuildSign(b *testing.B) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params, body := signFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.buildSign(params, body, Sign_Md5, Version_NewV1)
	}
}

// BenchmarkBuildSign_HMAC 度量 HMAC-SHA256 签名计算性能。
func BenchmarkBuildSign_HMAC(b *testing.B) {
	s := BuildHttpSign("sign", "mysecret", 0, "timestamp", "", "1")
	params, body := signFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.buildSign(params, body, Sign_HMacSha256, Version_NewV1)
	}
}

// BenchmarkMap2uri 度量参数序列化性能。
func BenchmarkMap2uri(b *testing.B) {
	params, _ := signFixture()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Map2uri(params, "sign", true, false)
	}
}

// BenchmarkCheckSign 度量完整校验链路（含时间戳解析与比对）的性能。
func BenchmarkCheckSign(b *testing.B) {
	s := BuildHttpSign("sign", "mysecret", 60, "timestamp", "", "1")
	params, body := signFixture()
	params["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	s.PushSign(params, body)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CheckSign(params, body, "")
	}
}
