package http_sign

// HTTPSignApi config struct

type Config struct {
	IndexName     string `json:"index_name" yaml:"index_name"`
	PrivateKey    string `json:"private_key" yaml:"private_key"`
	SignName      string `json:"sign_name" yaml:"sign_name"`
	ExpiredTime   int    `json:"expired_time" yaml:"expired_time"`
	TimestampName string `json:"timestamp_name" yaml:"timestamp_name"`
	SignType      string `json:"sign_type" yaml:"sign_type"`
	RequestIDName string `json:"request_id_name" yaml:"request_id_name"`
	VersionType   string `json:"version_type" yaml:"version_type"`
}
