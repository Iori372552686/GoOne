package redis

import (
	"crypto/tls"
	"reflect"
	"testing"
)

func TestBuildTLSConfig(t *testing.T) {
	got, err := buildTLSConfig(TLSConfig{Enabled: true, ServerName: "redis.internal", InsecureSkipVerify: true})
	if err != nil {
		t.Fatalf("buildTLSConfig() error = %v", err)
	}
	if got == nil || got.ServerName != "redis.internal" || !got.InsecureSkipVerify || got.MinVersion != tls.VersionTLS12 {
		t.Fatalf("buildTLSConfig() = %#v", got)
	}
}

func TestConfigNormalizeLegacyStandalone(t *testing.T) {
	got, err := (Config{
		InstanceID: 1,
		IP:         "127.0.0.1",
		Port:       6379,
		DbIndex:    3,
	}).normalize()
	if err != nil {
		t.Fatalf("normalize() error = %v", err)
	}

	if got.Mode != ModeStandalone {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeStandalone)
	}
	if want := []string{"127.0.0.1:6379"}; !reflect.DeepEqual(got.Addresses, want) {
		t.Fatalf("Addresses = %#v, want %#v", got.Addresses, want)
	}
	if got.PoolSize != defaultPoolSize {
		t.Fatalf("PoolSize = %d, want %d", got.PoolSize, defaultPoolSize)
	}
	if got.DbIndex != 3 {
		t.Fatalf("DbIndex = %d, want 3", got.DbIndex)
	}
}

func TestConfigNormalizeNewFieldsTakePrecedence(t *testing.T) {
	got, err := (Config{
		IP:        "legacy-host",
		Port:      6379,
		IsCluster: false,
		Mode:      string(ModeCluster),
		Addresses: []string{"redis-1:6379", "redis-2:6379"},
		PoolSize:  16,
	}).normalize()
	if err != nil {
		t.Fatalf("normalize() error = %v", err)
	}

	if got.Mode != ModeCluster {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeCluster)
	}
	if want := []string{"redis-1:6379", "redis-2:6379"}; !reflect.DeepEqual(got.Addresses, want) {
		t.Fatalf("Addresses = %#v, want %#v", got.Addresses, want)
	}
	if got.PoolSize != 16 {
		t.Fatalf("PoolSize = %d, want 16", got.PoolSize)
	}
}

func TestConfigNormalizeRejectsClusterDatabase(t *testing.T) {
	_, err := (Config{
		Mode:      string(ModeCluster),
		Addresses: []string{"redis-1:6379"},
		DbIndex:   1,
	}).normalize()
	if err == nil {
		t.Fatal("normalize() error = nil, want cluster db_index validation error")
	}
}

func TestConfigNormalizeRequiresSentinelMasterName(t *testing.T) {
	_, err := (Config{
		Mode:      string(ModeSentinel),
		Addresses: []string{"sentinel-1:26379"},
	}).normalize()
	if err == nil {
		t.Fatal("normalize() error = nil, want sentinel master_name validation error")
	}
}

func TestConfigNormalizeRejectsIncompleteTLSClientCertificate(t *testing.T) {
	_, err := (Config{
		IP:   "127.0.0.1",
		Port: 6379,
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "client.crt",
		},
	}).normalize()
	if err == nil {
		t.Fatal("normalize() error = nil, want TLS cert/key validation error")
	}
}
