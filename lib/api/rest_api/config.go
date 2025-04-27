package rest_api

// ServiceAPI config struct
type Config struct {
	ServiceName string   `json:"service_name" yaml:"service_name"`
	Urls        []string `json:"urls"         yaml:"urls"`
	User        string   `json:"user"         yaml:"user"`
	Pass        string   `json:"pass"         yaml:"pass"`
	SignName    string   `json:"sign_name"    yaml:"sign_name"`
}

/**
 * UrlConfig
 * @Description:
**/
type UrlConfig struct {
	Urls     []string
	UrlCount int64
}
