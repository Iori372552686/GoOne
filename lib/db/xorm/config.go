package orm

import "strings"

// DbInfo
type DbInfo struct {
	IP       string `json:"ip" yaml:"ip"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password"  yaml:"password"`
	DBName   string `json:"db_name"  yaml:"db_name"`
}

// xorm db config struct
type Config struct {
	InstanceID  int       `json:"instance_id" yaml:"instance_id"`
	IndexName   string    `json:"index_name" yaml:"index_name"`
	Master      *DbInfo   `json:"master" yaml:"master"` //主库
	Slaves      []*DbInfo `json:"slaves" yaml:"slaves"` //从库
	Description string    `json:"description" yaml:"description"`
	MaxIdle     int       `json:"max_idle" yaml:"max_idle"`
	MaxOpen     int       `json:"max_open" yaml:"max_open"`
	ShowSQL     bool      `json:"show_sql" yaml:"show_sql"`
	InitFlag    bool      `json:"init_flag" yaml:"init_flag"`
	DriveName   string    `json:"drive_name" yaml:"drive_name"`
}

// redactDSN 去除 DSN 中的明文密码，仅保留可诊断信息。
// 输入形如 `user:password@tcp(ip:port)/db?...`，输出 `***@tcp(ip:port)/db?...`。
func redactDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	if at < 0 {
		return dsn
	}
	return "***" + dsn[at:]
}

// redactDSNs 批量脱敏，返回新切片，不改原切片。
func redactDSNs(dsns []string) []string {
	out := make([]string, len(dsns))
	for i, d := range dsns {
		out[i] = redactDSN(d)
	}
	return out
}
