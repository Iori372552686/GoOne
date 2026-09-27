package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type fakeUniversalClient struct {
	goredis.UniversalClient
	pingErr    error
	closeErr   error
	closeCount atomic.Int32
	hookCount  atomic.Int32
}

func (f *fakeUniversalClient) AddHook(goredis.Hook) { f.hookCount.Add(1) }

func (f *fakeUniversalClient) Ping(context.Context) *goredis.StatusCmd {
	return goredis.NewStatusResult("PONG", f.pingErr)
}

func (f *fakeUniversalClient) Close() error {
	f.closeCount.Add(1)
	return f.closeErr
}

func TestClientReturnsMissingInstanceError(t *testing.T) {
	_, err := NewRedisMgr().Client(42)
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("Client() error = %v, want ErrInstanceNotFound", err)
	}
}

func TestInitAndRunRollsBackCreatedClients(t *testing.T) {
	first := &fakeUniversalClient{}
	second := &fakeUniversalClient{pingErr: errors.New("ping failed")}
	clients := []*fakeUniversalClient{first, second}
	next := 0
	m := NewRedisMgr()
	m.newClient = func(normalizedConfig, *tls.Config) (goredis.UniversalClient, error) {
		client := clients[next]
		next++
		return client, nil
	}

	err := m.InitAndRun(context.Background(), []Config{
		{InstanceID: 1, IP: "127.0.0.1", Port: 6379},
		{InstanceID: 2, IP: "127.0.0.1", Port: 6380},
	})
	if err == nil {
		t.Fatal("InitAndRun() error = nil, want ping failure")
	}
	if got := m.InstanceCount(); got != 0 {
		t.Fatalf("InstanceCount() = %d, want 0 after rollback", got)
	}
	if got := first.closeCount.Load(); got != 1 {
		t.Fatalf("first close count = %d, want 1", got)
	}
	if got := second.closeCount.Load(); got != 1 {
		t.Fatalf("failed client close count = %d, want 1", got)
	}
	if first.hookCount.Load() != 1 || second.hookCount.Load() != 1 {
		t.Fatalf("metrics hook counts = %d/%d, want 1/1", first.hookCount.Load(), second.hookCount.Load())
	}
}

func TestCloseIsIdempotentForRegisteredClient(t *testing.T) {
	client := &fakeUniversalClient{}
	m := NewRedisMgr()
	m.newClient = func(normalizedConfig, *tls.Config) (goredis.UniversalClient, error) {
		return client, nil
	}
	if err := m.AddInstance(context.Background(), Config{InstanceID: 1, IP: "127.0.0.1", Port: 6379}); err != nil {
		t.Fatalf("AddInstance() error = %v", err)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if got := client.closeCount.Load(); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
}

func TestSetBytesHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := goredis.NewClient(&goredis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: time.Second,
	})
	t.Cleanup(func() { _ = client.Close() })
	m := NewRedisMgr()
	m.clients.Store(uint32(1), client)

	err := m.SetBytes(ctx, 1, "key", []byte("value"), 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SetBytes() error = %v, want context.Canceled", err)
	}
}

type fakePipeliner struct {
	goredis.Pipeliner
	hsetKey   string
	hsetValue map[string]interface{}
	expireKey string
	expireTTL time.Duration
	execErr   error
	execDone  bool
}

func (p *fakePipeliner) HSet(_ context.Context, key string, values ...interface{}) *goredis.IntCmd {
	p.hsetKey = key
	if len(values) > 0 {
		p.hsetValue, _ = values[0].(map[string]interface{})
	}
	return goredis.NewIntResult(1, nil)
}

func (p *fakePipeliner) Expire(_ context.Context, key string, ttl time.Duration) *goredis.BoolCmd {
	p.expireKey = key
	p.expireTTL = ttl
	return goredis.NewBoolResult(true, nil)
}

func (p *fakePipeliner) Exec(context.Context) ([]goredis.Cmder, error) {
	p.execDone = true
	return nil, p.execErr
}

type pipelineClient struct {
	fakeUniversalClient
	pipe *fakePipeliner
}

func (c *pipelineClient) TxPipeline() goredis.Pipeliner { return c.pipe }

func (c *pipelineClient) HSet(_ context.Context, _ string, _ ...interface{}) *goredis.IntCmd {
	return goredis.NewIntResult(1, nil)
}

func TestHSetFieldsExpirePipelinesHSetAndExpire(t *testing.T) {
	pipe := &fakePipeliner{}
	mgr := NewRedisMgr()
	mgr.AddClientInstance(1, &pipelineClient{pipe: pipe})

	err := mgr.HSetFieldsExpire(context.Background(), 1, "role:7",
		map[string][]byte{"basic": []byte("x")}, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("HSetFieldsExpire() error = %v", err)
	}
	if pipe.hsetKey != "role:7" || pipe.hsetValue == nil {
		t.Fatalf("HSet not piped: key=%q values=%v", pipe.hsetKey, pipe.hsetValue)
	}
	if pipe.expireKey != "role:7" || pipe.expireTTL != 30*24*time.Hour {
		t.Fatalf("Expire not piped: key=%q ttl=%v", pipe.expireKey, pipe.expireTTL)
	}
	if !pipe.execDone {
		t.Fatal("pipeline not executed")
	}
}

func TestHSetFieldsExpireZeroTTLFallsBackToPlainHSet(t *testing.T) {
	pipe := &fakePipeliner{}
	mgr := NewRedisMgr()
	mgr.AddClientInstance(1, &pipelineClient{pipe: pipe})

	if err := mgr.HSetFieldsExpire(context.Background(), 1, "role:7",
		map[string][]byte{"basic": []byte("x")}, 0); err != nil {
		t.Fatal(err)
	}
	if pipe.execDone {
		t.Fatal("ttl=0 应退化为 HSetFields，不走 pipeline")
	}
}

func TestHSetFieldsExpireEmptyMapIsNoop(t *testing.T) {
	pipe := &fakePipeliner{}
	mgr := NewRedisMgr()
	mgr.AddClientInstance(1, &pipelineClient{pipe: pipe})
	if err := mgr.HSetFieldsExpire(context.Background(), 1, "role:7", nil, time.Hour); err != nil {
		t.Fatal(err)
	}
	if pipe.execDone {
		t.Fatal("空 map 应为 no-op")
	}
}
