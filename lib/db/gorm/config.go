package gormdb

import (
	"fmt"
	"net"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

type DbInfo struct {
	IP       string `json:"ip" yaml:"ip"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
	DBName   string `json:"db_name" yaml:"db_name"`
}

// Config preserves the existing base_cfg.dependencies.orm_instances YAML contract.
type Config struct {
	InstanceID  int       `json:"instance_id" yaml:"instance_id"`
	IndexName   string    `json:"index_name" yaml:"index_name"`
	Master      *DbInfo   `json:"master" yaml:"master"`
	Slaves      []*DbInfo `json:"slaves" yaml:"slaves"`
	Description string    `json:"description" yaml:"description"`
	MaxIdle     int       `json:"max_idle" yaml:"max_idle"`
	MaxOpen     int       `json:"max_open" yaml:"max_open"`
	ShowSQL     bool      `json:"show_sql" yaml:"show_sql"`
	InitFlag    bool      `json:"init_flag" yaml:"init_flag"`
	DriveName   string    `json:"drive_name" yaml:"drive_name"`
}

func (c Config) validate() error {
	if strings.TrimSpace(c.IndexName) == "" {
		return errorsForConfig(c, "index_name is required")
	}
	driver := strings.ToLower(strings.TrimSpace(c.DriveName))
	if driver != "" && driver != "mysql" {
		return errorsForConfig(c, fmt.Sprintf("unsupported drive_name %q; only mysql is supported", c.DriveName))
	}
	if err := validateDbInfo(c.Master, "master"); err != nil {
		return errorsForConfig(c, err.Error())
	}
	for i, slave := range c.Slaves {
		if err := validateDbInfo(slave, fmt.Sprintf("slave[%d]", i)); err != nil {
			return errorsForConfig(c, err.Error())
		}
	}
	if c.MaxIdle < 0 || c.MaxOpen < 0 || (c.MaxOpen > 0 && c.MaxIdle > c.MaxOpen) {
		return errorsForConfig(c, "invalid max_idle/max_open")
	}
	return nil
}

func validateDbInfo(info *DbInfo, role string) error {
	if info == nil {
		return fmt.Errorf("%s is required", role)
	}
	if strings.TrimSpace(info.IP) == "" || info.Port <= 0 || strings.TrimSpace(info.User) == "" || strings.TrimSpace(info.DBName) == "" {
		return fmt.Errorf("%s requires ip, port, user and db_name", role)
	}
	return nil
}

func errorsForConfig(c Config, message string) error {
	return fmt.Errorf("gorm instance %q: %s", c.IndexName, message)
}

func buildDSN(info *DbInfo) string {
	config := mysqldriver.NewConfig()
	config.User = info.User
	config.Passwd = info.Password
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(info.IP, fmt.Sprint(info.Port))
	config.DBName = info.DBName
	config.ParseTime = true
	config.Loc = time.Local
	config.Timeout = 3 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.WriteTimeout = 15 * time.Second
	config.Params = map[string]string{"charset": "utf8mb4"}
	return config.FormatDSN()
}

func redactDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	if at < 0 {
		return dsn
	}
	return "***" + dsn[at:]
}
