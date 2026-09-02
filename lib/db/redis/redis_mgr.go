package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/module/conf"
	goredis "github.com/redis/go-redis/v9"
)

const DefaultConfKey = "base_cfg.dependencies.db_instances"

var ErrInstanceNotFound = errors.New("redis instance not found")

type redisClientFactory func(normalizedConfig, *tls.Config) (goredis.UniversalClient, error)

type RedisMgr struct {
	clients   sync.Map // map[uint32]goredis.UniversalClient
	newClient redisClientFactory
}

func NewRedisMgr() *RedisMgr {
	return &RedisMgr{newClient: newUniversalClient}
}

func (m *RedisMgr) OnStart(ctx context.Context) error {
	var dbs []Config
	if err := conf.Unmarshal(DefaultConfKey, &dbs); err != nil {
		return err
	}
	if len(dbs) == 0 {
		return nil
	}
	return m.InitAndRun(ctx, dbs)
}

func (m *RedisMgr) OnStop(context.Context) error { return m.Close() }

func (m *RedisMgr) InitAndRun(ctx context.Context, configs []Config) error {
	safe := make([]string, len(configs))
	for i, config := range configs {
		safe[i] = config.SafeString()
	}
	logger.Infof("RedisMgr InsInit.. | %v", safe)

	added := make([]uint32, 0, len(configs))
	for _, config := range configs {
		if err := m.AddInstance(ctx, config); err != nil {
			for i := len(added) - 1; i >= 0; i-- {
				_ = m.closeInstance(added[i])
			}
			return err
		}
		added = append(added, uint32(config.InstanceID))
	}
	logger.Infof("RedisMgr InsInit... Done !")
	return nil
}

func (m *RedisMgr) AddInstance(ctx context.Context, config Config) error {
	normalized, err := config.normalize()
	if err != nil {
		return err
	}
	tlsConfig, err := buildTLSConfig(normalized.TLS)
	if err != nil {
		return fmt.Errorf("redis instance %d tls: %w", config.InstanceID, err)
	}
	client, err := m.newClient(normalized, tlsConfig)
	if err != nil {
		return fmt.Errorf("redis instance %d create client: %w", config.InstanceID, err)
	}
	instanceID := uint32(config.InstanceID)
	client.AddHook(redisMetricsHook{instanceID: instanceID})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("redis instance %d ping: %w", config.InstanceID, err)
	}

	if previous, loaded := m.clients.Swap(instanceID, client); loaded {
		logger.Warningf("overwrite redis instance %d", instanceID)
		if oldClient, ok := previous.(goredis.UniversalClient); ok && oldClient != nil {
			_ = oldClient.Close()
		}
	}
	return nil
}

func (m *RedisMgr) Client(instanceID uint32) (goredis.UniversalClient, error) {
	if m == nil {
		return nil, fmt.Errorf("%w: %d", ErrInstanceNotFound, instanceID)
	}
	value, ok := m.clients.Load(instanceID)
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrInstanceNotFound, instanceID)
	}
	client, ok := value.(goredis.UniversalClient)
	if !ok || client == nil {
		return nil, fmt.Errorf("%w: %d", ErrInstanceNotFound, instanceID)
	}
	return client, nil
}

func (m *RedisMgr) InstanceCount() int {
	if m == nil {
		return 0
	}
	count := 0
	m.clients.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (m *RedisMgr) Close() error {
	if m == nil {
		return nil
	}
	var errs []error
	m.clients.Range(func(key, _ any) bool {
		instanceID := key.(uint32)
		if err := m.closeInstance(instanceID); err != nil {
			errs = append(errs, fmt.Errorf("redis instance %d close: %w", instanceID, err))
		}
		return true
	})
	return errors.Join(errs...)
}

func (m *RedisMgr) closeInstance(instanceID uint32) error {
	value, ok := m.clients.LoadAndDelete(instanceID)
	if !ok {
		return nil
	}
	client, ok := value.(goredis.UniversalClient)
	if !ok || client == nil {
		return nil
	}
	return client.Close()
}

func (m *RedisMgr) SetBytes(ctx context.Context, instanceID uint32, key string, value []byte, ttl time.Duration) error {
	client, err := m.Client(instanceID)
	if err != nil {
		return err
	}
	return client.Set(ctx, key, value, ttl).Err()
}

func (m *RedisMgr) GetBytes(ctx context.Context, instanceID uint32, key string) ([]byte, error) {
	client, err := m.Client(instanceID)
	if err != nil {
		return nil, err
	}
	value, err := client.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	return value, err
}

func (m *RedisMgr) MGetBytes(ctx context.Context, instanceID uint32, keys ...string) ([][]byte, error) {
	client, err := m.Client(instanceID)
	if err != nil {
		return nil, err
	}
	values, err := client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	result := make([][]byte, len(values))
	for i, value := range values {
		switch typed := value.(type) {
		case nil:
		case string:
			result[i] = []byte(typed)
		case []byte:
			result[i] = append([]byte(nil), typed...)
		default:
			result[i] = []byte(fmt.Sprint(typed))
		}
	}
	return result, nil
}

func (m *RedisMgr) Delete(ctx context.Context, instanceID uint32, keys ...string) error {
	client, err := m.Client(instanceID)
	if err != nil {
		return err
	}
	return client.Del(ctx, keys...).Err()
}

func (m *RedisMgr) HSetBytes(ctx context.Context, instanceID uint32, key, field string, value []byte) error {
	client, err := m.Client(instanceID)
	if err != nil {
		return err
	}
	return client.HSet(ctx, key, field, value).Err()
}

func (m *RedisMgr) HGetBytes(ctx context.Context, instanceID uint32, key, field string) ([]byte, error) {
	client, err := m.Client(instanceID)
	if err != nil {
		return nil, err
	}
	value, err := client.HGet(ctx, key, field).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	return value, err
}

func (m *RedisMgr) HGetAllBytes(ctx context.Context, instanceID uint32, key string) (map[string][]byte, error) {
	client, err := m.Client(instanceID)
	if err != nil {
		return nil, err
	}
	values, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string][]byte, len(values))
	for field, value := range values {
		result[field] = []byte(value)
	}
	return result, nil
}

func (m *RedisMgr) IncrBy(ctx context.Context, instanceID uint32, key string, value int64) (int64, error) {
	client, err := m.Client(instanceID)
	if err != nil {
		return 0, err
	}
	return client.IncrBy(ctx, key, value).Result()
}

func (m *RedisMgr) ZAdd(ctx context.Context, instanceID uint32, key string, members ...goredis.Z) error {
	client, err := m.Client(instanceID)
	if err != nil {
		return err
	}
	return client.ZAdd(ctx, key, members...).Err()
}

func (m *RedisMgr) ZRange(ctx context.Context, instanceID uint32, key string, start, stop int64) ([]string, error) {
	client, err := m.Client(instanceID)
	if err != nil {
		return nil, err
	}
	return client.ZRange(ctx, key, start, stop).Result()
}

func buildTLSConfig(config TLSConfig) (*tls.Config, error) {
	if !config.Enabled {
		return nil, nil
	}
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         config.ServerName,
		InsecureSkipVerify: config.InsecureSkipVerify, //nolint:gosec // explicit deployment option
	}
	if config.CAFile != "" {
		pem, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca_file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("ca_file contains no valid certificates")
		}
		tlsConfig.RootCAs = roots
	}
	if config.CertFile != "" {
		certificate, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}

func newUniversalClient(config normalizedConfig, tlsConfig *tls.Config) (goredis.UniversalClient, error) {
	dialTimeout := durationMS(config.DialTimeoutMS)
	readTimeout := durationMS(config.ReadTimeoutMS)
	writeTimeout := durationMS(config.WriteTimeoutMS)
	poolTimeout := durationMS(config.PoolTimeoutMS)

	switch config.Mode {
	case ModeStandalone:
		return goredis.NewClient(&goredis.Options{
			Addr: config.Addresses[0], Username: config.Username, Password: config.Password,
			DB: config.DbIndex, PoolSize: config.PoolSize, MinIdleConns: config.MinIdleConns,
			DialTimeout: dialTimeout, ReadTimeout: readTimeout, WriteTimeout: writeTimeout,
			PoolTimeout: poolTimeout, TLSConfig: tlsConfig,
		}), nil
	case ModeSentinel:
		return goredis.NewFailoverClient(&goredis.FailoverOptions{
			MasterName: config.MasterName, SentinelAddrs: config.Addresses,
			SentinelUsername: config.SentinelUsername, SentinelPassword: config.SentinelPassword,
			Username: config.Username, Password: config.Password, DB: config.DbIndex,
			PoolSize: config.PoolSize, MinIdleConns: config.MinIdleConns,
			DialTimeout: dialTimeout, ReadTimeout: readTimeout, WriteTimeout: writeTimeout,
			PoolTimeout: poolTimeout, TLSConfig: tlsConfig,
		}), nil
	case ModeCluster:
		return goredis.NewClusterClient(&goredis.ClusterOptions{
			Addrs: config.Addresses, Username: config.Username, Password: config.Password,
			PoolSize: config.PoolSize, MinIdleConns: config.MinIdleConns,
			DialTimeout: dialTimeout, ReadTimeout: readTimeout, WriteTimeout: writeTimeout,
			PoolTimeout: poolTimeout, TLSConfig: tlsConfig,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported redis mode %q", config.Mode)
	}
}

func durationMS(value int) time.Duration {
	if value == 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}
