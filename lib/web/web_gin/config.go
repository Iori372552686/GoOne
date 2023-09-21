package web_gin

type Config struct {
	IP          string `json:"ip" yaml:"ip"`                     // ip addr
	Port        int    `json:"port" yaml:"port"`                 // port 端口
	SessionName string `json:"session_name" yaml:"session_name"` //session名
	AuthEnable  bool   `json:"auth_enable" yaml:"auth_enable"`   //签名开关
	Mode        string `json:"mode" yaml:"mode"`                 //http模式
}
