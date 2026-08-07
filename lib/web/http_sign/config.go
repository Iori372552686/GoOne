package http_sign

// Config 描述一个从服务配置加载的、具名的 HttpSign 实例。
//
// SignType 选择签名算法："md5"（默认）、"sha1" 或 "hmac_sha256"
// （新部署推荐）。
type Config struct {
	IndexName     string `json:"index_name" yaml:"index_name"`           // 注册键名
	PrivateKey    string `json:"private_key" yaml:"private_key"`         // 共享密钥 / HMAC 密钥
	SignName      string `json:"sign_name" yaml:"sign_name"`             // 签名字段名
	ExpiredTime   int    `json:"expired_time" yaml:"expired_time"`       // 有效期（秒）；0 表示不校验
	TimestampName string `json:"timestamp_name" yaml:"timestamp_name"`   // 时间戳字段名
	SignType      string `json:"sign_type" yaml:"sign_type"`             // "md5" | "sha1" | "hmac_sha256"
	RequestIDName string `json:"request_id_name" yaml:"request_id_name"` // 请求唯一标识字段名；空串表示不启用
	VersionType   string `json:"version_type" yaml:"version_type"`       // 保留字段，仅为兼容
}
