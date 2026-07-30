package web_gin

import "time"

type Config struct {
	IP          string `json:"ip" yaml:"ip"`                     // ip addr
	Port        int    `json:"port" yaml:"port"`                 // port 端口
	SessionName string `json:"session_name" yaml:"session_name"` //session名
	AuthEnable  bool   `json:"auth_enable" yaml:"auth_enable"`   //签名开关
	Mode        string `json:"mode" yaml:"mode"`                 //http模式

	// HTTP 超时与大小限制（V3-P0-06：防止 Slowloris 与超大请求耗尽资源）。
	// 零值表示不设置对应限制（保持向后兼容）；生产配置应显式设置。
	ReadHeaderTimeout time.Duration `json:"read_header_timeout" yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout      time.Duration `json:"write_timeout" yaml:"write_timeout"`
	IdleTimeout       time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	MaxHeaderBytes    int           `json:"max_header_bytes" yaml:"max_header_bytes"`
}
