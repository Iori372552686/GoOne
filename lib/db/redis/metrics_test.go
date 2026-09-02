package redis

import (
	"testing"

	goredis "github.com/redis/go-redis/v9"
)

func TestRedisNilIsNotReportedAsOperationalError(t *testing.T) {
	if got := redisResultLabel(goredis.Nil); got != "ok" {
		t.Fatalf("redisResultLabel(redis.Nil) = %q, want ok", got)
	}
}
