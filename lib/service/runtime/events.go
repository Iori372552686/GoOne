package runtime

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
)

// 结构化生命周期事件名。
//
// 这些事件名作为日志消息的前缀（[event:xxx]），使运维可按事件名精确 grep 与告警，
// 取代散落的 Infof/Errorf。所有生命周期关键节点都应经 logEvent 上报。
const (
	eventComponentStarting    = "component_starting"
	eventComponentStarted     = "component_started"
	eventComponentStartFailed = "component_start_failed"
	eventComponentStopFailed  = "component_stop_failed"
	eventStateChanged         = "state_changed"
	eventDrainStarted         = "drain_started"
	eventDrainCompleted       = "drain_completed"
	eventDrainTimedOut        = "drain_timed_out"
	eventDrainEscalated       = "drain_escalated"
)

// logEvent 记录一条结构化生命周期事件（Info 级）。
// 格式：[event:<name>] <service> <detail>
func logEvent(name, service, detail string) {
	logger.Infof("[event:%s] %s %s", name, service, detail)
}

// logEventf 记录一条带格式化 detail 的结构化事件（Info 级）。
func logEventf(name, service, format string, args ...any) {
	logger.Infof("[event:%s] %s "+format, append([]any{name, service}, args...)...)
}

// logEventError 记录一条结构化事件（Error 级，用于失败事件）。
func logEventError(name, service string, err error) {
	logger.Errorf("[event:%s] %s | %v", name, service, err)
}
