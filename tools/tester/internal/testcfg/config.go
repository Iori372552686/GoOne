// Package testcfg 统一测试配置：单元/回归测试与全流程压力测试共用一份 tester.toml。
//
// 配置分层：
//   - [run]      模式开关：regression（单元/回归）| stress（全流程压测）
//   - [server]   被测服务器地址（TCP/WS 网关 + pprof）
//   - [player]   模拟玩家账号与数量
//   - [stress]   压测开关：随机/固定顺序、循环、时长、上线速率
//   - [modules.<name>] 业务模块覆盖开关（enabled/weight/order + 模块私有参数）
//   - [collect]  指标采样与报告输出
//
// 基础流程（连接、登录）不在 [modules] 开关内，始终执行。
package testcfg

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	// RunModeRegression 单元/回归测试模式：跑完整用例集（含边界用例），输出 PASS/FAIL 汇总。
	RunModeRegression = "regression"
	// RunModeStress 全流程压力测试模式：只跑正常路径操作集，循环模拟并输出压测报告。
	RunModeStress = "stress"

	// FlowRandom 随机业务模拟：按模块权重随机挑选。
	FlowRandom = "random"
	// FlowSequential 固定顺序业务模拟：按模块 order 轮转。
	FlowSequential = "sequential"
)

type RunConfig struct {
	Mode string `toml:"mode"`
}

type ServerConfig struct {
	Host      string `toml:"host"`
	TcpPort   int    `toml:"tcp_port"`
	WsPort    int    `toml:"ws_port"`
	WsPath    string `toml:"ws_path"`
	PprofPort int    `toml:"pprof_port"`
	Transport string `toml:"transport"` // tcp | ws
}

// TcpAddr 返回 TCP 网关地址，如 127.0.0.1:11002。
func (s *ServerConfig) TcpAddr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.TcpPort)
}

// WsURL 返回 WebSocket 网关地址，如 ws://127.0.0.1:11001/ws。
func (s *ServerConfig) WsURL() string {
	path := s.WsPath
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("ws://%s:%d%s", s.Host, s.WsPort, path)
}

// PprofBaseURL pprof HTTP 根地址，如 http://127.0.0.1:6060。
func (s *ServerConfig) PprofBaseURL() string {
	return fmt.Sprintf("http://%s:%d", s.Host, s.PprofPort)
}

type PlayerConfig struct {
	Players       int    `toml:"players"`
	StartUID      int64  `toml:"start_uid"`
	Channel       string `toml:"channel"`
	AccountPrefix string `toml:"account_prefix"`
	DevicePrefix  string `toml:"device_prefix"`
	Token         string `toml:"token"`
}

type StressConfig struct {
	Flow         string `toml:"flow"`            // random | sequential
	Loop         bool   `toml:"loop"`            // true=循环执行直到 duration/stop；false=每玩家单轮后结束
	Duration     string `toml:"duration"`        // "10m"；"0" 表示无限直到手动 stop
	RampUpPerSec int    `toml:"ramp_up_per_sec"` // 每秒上线玩家数
	ThinkTimeMs  int    `toml:"think_time_ms"`   // 每轮业务间隔
}

// DurationParsed 解析压测时长；0 表示无限。
func (s *StressConfig) DurationParsed() time.Duration {
	d, err := time.ParseDuration(s.Duration)
	if err != nil {
		return 0
	}
	return d
}

// ModuleSetting 单个业务模块的覆盖开关（通用部分）。
// 模块私有参数保留在原始 TOML 段中，由组件通过 Config.DecodeModule 解码。
type ModuleSetting struct {
	Enabled bool `toml:"enabled"`
	Weight  int  `toml:"weight"` // random 流程使用；<=0 视为 1
	Order   int  `toml:"order"`  // sequential 流程使用；小的先执行
}

type CollectConfig struct {
	SampleInterval  string `toml:"sample_interval"`
	ProfileInterval string `toml:"profile_interval"` // "0" 关闭 pprof 存档
	ReportDir       string `toml:"report_dir"`
}

func (c *CollectConfig) SampleIntervalParsed() time.Duration {
	d, err := time.ParseDuration(c.SampleInterval)
	if err != nil || d <= 0 {
		return 5 * time.Second
	}
	return d
}

func (c *CollectConfig) ProfileIntervalParsed() time.Duration {
	d, err := time.ParseDuration(c.ProfileInterval)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

type Config struct {
	Run     RunConfig                 `toml:"run"`
	Server  ServerConfig              `toml:"server"`
	Player  PlayerConfig              `toml:"player"`
	Stress  StressConfig              `toml:"stress"`
	Modules map[string]toml.Primitive `toml:"modules"`
	Collect CollectConfig             `toml:"collect"`

	meta     toml.MetaData
	settings map[string]ModuleSetting
}

// Load 读取并解析 tester.toml，应用默认值与 GOONE_TESTER_* 环境变量覆盖。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := defaultConfig()
	meta, err := toml.Decode(string(data), cfg)
	if err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	cfg.meta = meta

	cfg.settings = make(map[string]ModuleSetting, len(cfg.Modules))
	for name, prim := range cfg.Modules {
		s := ModuleSetting{Weight: 1}
		if err := meta.PrimitiveDecode(prim, &s); err != nil {
			return nil, fmt.Errorf("parse [modules.%s]: %w", name, err)
		}
		if s.Weight <= 0 {
			s.Weight = 1
		}
		cfg.settings[name] = s
	}

	applyEnvOverrides(cfg)

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Run: RunConfig{Mode: RunModeRegression},
		Server: ServerConfig{
			Host:      "127.0.0.1",
			TcpPort:   11002,
			WsPort:    11001,
			WsPath:    "/",
			PprofPort: 6060,
			Transport: "tcp",
		},
		Player: PlayerConfig{
			Players:       1,
			StartUID:      100001,
			Channel:       "tester",
			AccountPrefix: "tester_acc",
			DevicePrefix:  "tester_device",
			Token:         "",
		},
		Stress: StressConfig{
			Flow:         FlowRandom,
			Loop:         true,
			Duration:     "10m",
			RampUpPerSec: 20,
			ThinkTimeMs:  200,
		},
		Collect: CollectConfig{
			SampleInterval:  "5s",
			ProfileInterval: "30s",
			ReportDir:       "./report",
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_ACCOUNT_PREFIX")); v != "" {
		cfg.Player.AccountPrefix = v
	}
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_DEVICE_PREFIX")); v != "" {
		cfg.Player.DevicePrefix = v
	}
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_START_UID")); v != "" {
		if uid, err := strconv.ParseInt(v, 10, 64); err == nil && uid > 0 {
			cfg.Player.StartUID = uid
		}
	}
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_MODE")); v != "" {
		cfg.Run.Mode = v
	}
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_PLAYERS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Player.Players = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_TRANSPORT")); v != "" {
		cfg.Server.Transport = v
	}
}

func (c *Config) validate() error {
	switch c.Run.Mode {
	case RunModeRegression, RunModeStress:
	default:
		return fmt.Errorf("invalid [run].mode %q (want %q or %q)", c.Run.Mode, RunModeRegression, RunModeStress)
	}
	switch c.Stress.Flow {
	case FlowRandom, FlowSequential:
	default:
		return fmt.Errorf("invalid [stress].flow %q (want %q or %q)", c.Stress.Flow, FlowRandom, FlowSequential)
	}
	if c.Player.Players <= 0 {
		return fmt.Errorf("[player].players must be > 0")
	}
	if len(c.EnabledModules()) == 0 {
		return fmt.Errorf("no enabled module in [modules]; enable at least one, e.g.\n[modules.login]\nenabled = true")
	}
	return nil
}

// EnabledModules 返回打开的业务模块名，按 order 升序（order 相同按名称稳定排序）。
func (c *Config) EnabledModules() []string {
	names := make([]string, 0, len(c.settings))
	for name, s := range c.settings {
		if s.Enabled {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		si, sj := c.settings[names[i]], c.settings[names[j]]
		if si.Order != sj.Order {
			return si.Order < sj.Order
		}
		return names[i] < names[j]
	})
	return names
}

// ModuleSetting 返回模块通用开关；未配置的模块返回零值（enabled=false）。
func (c *Config) ModuleSetting(name string) ModuleSetting {
	return c.settings[name]
}

// DecodeModule 将 [modules.<name>] 段解码到模块私有配置结构（组件细项参数）。
// 模块未配置时不报错、保持 v 原值。
func (c *Config) DecodeModule(name string, v any) error {
	prim, ok := c.Modules[name]
	if !ok {
		return nil
	}
	return c.meta.PrimitiveDecode(prim, v)
}

// ConfigPath 解析配置文件路径：flag/env 优先，否则取 etcDir 下的 tester.toml。
func ConfigPath(explicit, etcDir string) string {
	if explicit != "" {
		return explicit
	}
	if v := strings.TrimSpace(os.Getenv("GOONE_TESTER_CONFIG")); v != "" {
		return v
	}
	return etcDir + "/tester.toml"
}
