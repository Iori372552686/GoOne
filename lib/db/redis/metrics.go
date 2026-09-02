package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	goredis "github.com/redis/go-redis/v9"
)

type redisMetricsHook struct {
	instanceID uint32
}

func (h redisMetricsHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }

func (h redisMetricsHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		finish := beginRedisObserve(h.instanceID, cmd.Name())
		err := next(ctx, cmd)
		finish(err)
		return err
	}
}

func (h redisMetricsHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		finishes := make([]func(error), len(cmds))
		for i, cmd := range cmds {
			finishes[i] = beginRedisObserve(h.instanceID, cmd.Name())
		}
		err := next(ctx, cmds)
		for i, cmd := range cmds {
			cmdErr := cmd.Err()
			if cmdErr == nil && err != nil {
				cmdErr = err
			}
			finishes[i](cmdErr)
		}
		return err
	}
}

var redisDurationBuckets = []float64{
	0.0005,
	0.001,
	0.005,
	0.01,
	0.025,
	0.05,
	0.1,
	0.25,
	0.5,
	1,
	2.5,
}

var (
	redisCommandsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_redis_commands_total",
		Help: "Total Redis commands by instance, command, and result.",
	}, []string{"instance", "cmd", "result"})
	redisCommandDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "goone_redis_command_duration_seconds",
		Help:    "Latency distribution of Redis commands.",
		Buckets: redisDurationBuckets,
	}, []string{"instance", "cmd"})
	redisCommandErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_redis_command_errors_total",
		Help: "Total Redis command errors.",
	}, []string{"instance", "cmd"})
	redisCommandTimeouts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "goone_redis_command_timeouts_total",
		Help: "Total Redis command timeouts.",
	}, []string{"instance", "cmd"})
	redisCommandsInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "goone_redis_commands_in_flight",
		Help: "Current in-flight Redis commands.",
	}, []string{"instance", "cmd"})
)

func beginRedisObserve(instID uint32, cmd string) func(err error) {
	instanceLabel := fmt.Sprintf("%d", instID)
	cmdLabel := normalizeRedisCmd(cmd)
	redisCommandsInFlight.WithLabelValues(instanceLabel, cmdLabel).Inc()
	start := time.Now()
	return func(err error) {
		err = redisOperationalError(err)
		redisCommandsInFlight.WithLabelValues(instanceLabel, cmdLabel).Dec()
		redisCommandsTotal.WithLabelValues(instanceLabel, cmdLabel, redisResultLabel(err)).Inc()
		redisCommandDuration.WithLabelValues(instanceLabel, cmdLabel).Observe(time.Since(start).Seconds())
		if err != nil {
			redisCommandErrors.WithLabelValues(instanceLabel, cmdLabel).Inc()
			if redisIsTimeoutErr(err) {
				redisCommandTimeouts.WithLabelValues(instanceLabel, cmdLabel).Inc()
			}
		}
	}
}

func normalizeRedisCmd(cmd string) string {
	label := strings.TrimSpace(strings.ToUpper(cmd))
	if label == "" {
		return "UNKNOWN"
	}
	return label
}

func redisResultLabel(err error) string {
	err = redisOperationalError(err)
	if err == nil {
		return "ok"
	}
	if redisIsTimeoutErr(err) {
		return "timeout"
	}
	return "error"
}

func redisOperationalError(err error) error {
	if errors.Is(err, goredis.Nil) {
		return nil
	}
	return err
}

func redisIsTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}
