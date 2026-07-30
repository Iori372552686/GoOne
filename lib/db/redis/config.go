package redis

import "fmt"

// db config struct
type Config struct {
	InstanceID  int    `json:"instance_id" yaml:"instance_id"`
	IP          string `json:"ip" yaml:"ip"`
	Port        int    `json:"port" yaml:"port"`
	Password    string `json:"password" yaml:"password"`
	IsCluster   bool   `json:"is_cluster" yaml:"is_cluster"`
	DbIndex     int    `json:"db_index" yaml:"db_index"`
	Description string `json:"description" yaml:"description"`
}

// SafeString 返回不含 Password 的可日志表示，仅记录实例 ID、地址、DB 与集群标记，
// 避免凭据落盘。
func (c Config) SafeString() string {
	return fmt.Sprintf("{instance:%d addr:%s:%d db:%d cluster:%v desc:%q}",
		c.InstanceID, c.IP, c.Port, c.DbIndex, c.IsCluster, c.Description)
}
