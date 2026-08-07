package rest_api

// Config 描述单个 RestApi 实例的装配配置，由 RestApiMgr.Init 逐项解析。
//
// 字段契约：
//   - ServiceName：注册到 mgr 的 key，同时作为日志/错误归属；空则跳过该实例；
//   - Urls：后端地址列表，可多元素做分片，URL 形如 "https://host/path?"（问号属配置，由 Map2uri 直接拼接 query）；
//   - SignName：可选，指向 http_sign.SignMgr 中的 HttpSign 实例名，缺省则该实例不做签名；
//   - Timeout：兜底超时（秒），仅当调用方 ctx 无 deadline 时生效，<=0 表示复用底层默认 8s。
type Config struct {
	ServiceName string   `json:"service_name" yaml:"service_name"`
	Urls        []string `json:"urls"         yaml:"urls"`
	SignName    string   `json:"sign_name"    yaml:"sign_name"`
	Timeout     int      `json:"timeout"      yaml:"timeout"`
}
