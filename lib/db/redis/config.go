package redis

import (
	"fmt"
	"net"
	"strings"
)

const defaultPoolSize = 100

type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeSentinel   Mode = "sentinel"
	ModeCluster    Mode = "cluster"
)

type TLSConfig struct {
	Enabled            bool   `json:"enabled" yaml:"enabled"`
	ServerName         string `json:"server_name" yaml:"server_name"`
	CAFile             string `json:"ca_file" yaml:"ca_file"`
	CertFile           string `json:"cert_file" yaml:"cert_file"`
	KeyFile            string `json:"key_file" yaml:"key_file"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
}

// Config keeps the legacy ip/port/is_cluster fields for YAML compatibility.
// Explicit mode and addresses take precedence when both forms are present.
type Config struct {
	InstanceID       int       `json:"instance_id" yaml:"instance_id"`
	IP               string    `json:"ip" yaml:"ip"`
	Port             int       `json:"port" yaml:"port"`
	Password         string    `json:"password" yaml:"password"`
	IsCluster        bool      `json:"is_cluster" yaml:"is_cluster"`
	DbIndex          int       `json:"db_index" yaml:"db_index"`
	Description      string    `json:"description" yaml:"description"`
	Mode             string    `json:"mode" yaml:"mode"`
	Addresses        []string  `json:"addresses" yaml:"addresses"`
	Username         string    `json:"username" yaml:"username"`
	MasterName       string    `json:"master_name" yaml:"master_name"`
	SentinelUsername string    `json:"sentinel_username" yaml:"sentinel_username"`
	SentinelPassword string    `json:"sentinel_password" yaml:"sentinel_password"`
	PoolSize         int       `json:"pool_size" yaml:"pool_size"`
	MinIdleConns     int       `json:"min_idle_conns" yaml:"min_idle_conns"`
	DialTimeoutMS    int       `json:"dial_timeout_ms" yaml:"dial_timeout_ms"`
	ReadTimeoutMS    int       `json:"read_timeout_ms" yaml:"read_timeout_ms"`
	WriteTimeoutMS   int       `json:"write_timeout_ms" yaml:"write_timeout_ms"`
	PoolTimeoutMS    int       `json:"pool_timeout_ms" yaml:"pool_timeout_ms"`
	TLS              TLSConfig `json:"tls" yaml:"tls"`
}

type normalizedConfig struct {
	Config
	Mode      Mode
	Addresses []string
	PoolSize  int
}

func (c Config) normalize() (normalizedConfig, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(c.Mode)))
	if mode == "" {
		mode = ModeStandalone
		if c.IsCluster {
			mode = ModeCluster
		}
	}
	if mode != ModeStandalone && mode != ModeSentinel && mode != ModeCluster {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: unsupported mode %q", c.InstanceID, c.Mode)
	}

	addresses := make([]string, 0, len(c.Addresses))
	for _, address := range c.Addresses {
		if address = strings.TrimSpace(address); address != "" {
			addresses = append(addresses, address)
		}
	}
	if len(addresses) == 0 && strings.TrimSpace(c.IP) != "" && c.Port > 0 {
		addresses = append(addresses, net.JoinHostPort(strings.TrimSpace(c.IP), fmt.Sprint(c.Port)))
	}
	if len(addresses) == 0 {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: addresses or legacy ip/port is required", c.InstanceID)
	}
	if mode == ModeStandalone && len(addresses) != 1 {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: standalone mode requires exactly one address", c.InstanceID)
	}
	if mode == ModeCluster && c.DbIndex != 0 {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: cluster mode requires db_index=0", c.InstanceID)
	}
	if mode == ModeSentinel && strings.TrimSpace(c.MasterName) == "" {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: sentinel mode requires master_name", c.InstanceID)
	}

	poolSize := c.PoolSize
	if poolSize == 0 {
		poolSize = defaultPoolSize
	}
	if poolSize < 0 || c.MinIdleConns < 0 || c.MinIdleConns > poolSize {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: invalid pool_size/min_idle_conns", c.InstanceID)
	}
	if c.DialTimeoutMS < 0 || c.ReadTimeoutMS < 0 || c.WriteTimeoutMS < 0 || c.PoolTimeoutMS < 0 {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: timeouts cannot be negative", c.InstanceID)
	}
	if (c.TLS.CertFile == "") != (c.TLS.KeyFile == "") {
		return normalizedConfig{}, fmt.Errorf("redis instance %d: tls cert_file and key_file must be configured together", c.InstanceID)
	}

	return normalizedConfig{Config: c, Mode: mode, Addresses: addresses, PoolSize: poolSize}, nil
}

// SafeString returns a log representation without passwords or TLS private-key paths.
func (c Config) SafeString() string {
	normalized, err := c.normalize()
	if err != nil {
		return fmt.Sprintf("{instance:%d invalid:%q desc:%q}", c.InstanceID, err.Error(), c.Description)
	}
	return fmt.Sprintf("{instance:%d mode:%s addrs:%v db:%d pool:%d desc:%q}",
		c.InstanceID, normalized.Mode, normalized.Addresses, c.DbIndex, normalized.PoolSize, c.Description)
}
