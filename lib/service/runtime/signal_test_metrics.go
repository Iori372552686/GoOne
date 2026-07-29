package runtime

import (
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// readDrainTimeoutsMetric 读取 goone_drain_timeouts_total 的当前累计值，供测试断言
// 排空超时计数变化。
//
// 说明：promauto 在包初始化时把计数器注册到默认 registry；testutil.ToFloat64 直接
// 读取 collector 当前值，无需重新注册或 reset。
func readDrainTimeoutsMetric() int64 {
	return int64(testutil.ToFloat64(drainTimeouts))
}
