package http_sign

// HTTPSignApi config struct

type Config struct {
	IndexName     string `json:"IndexName" yaml:"IndexName"`
	PrivateKey    string `json:"PrivateKey" yaml:"PrivateKey"`
	SignName      string `json:"SignName" yaml:"SignName"`
	ExpiredTime   int    `json:"ExpiredTime" yaml:"ExpiredTime"`
	TimestampName string `json:"TimestampName" yaml:"TimestampName"`
	SignType      string `json:"SignType" yaml:"SignType"`
	RequestIDName string `json:"RequestIDName" yaml:"RequestIDName"`
	VersionType   string `json:"VersionType" yaml:"VersionType"`
}
