package runtime

import "testing"

// TestEventNamesDefined 验证 roadmap 要求的结构化事件名常量都已定义且拼写正确。
// 这些事件名用于日志前缀 [event:xxx]，运维按名 grep/告警。
func TestEventNamesDefined(t *testing.T) {
	want := []string{
		"component_starting",
		"component_started",
		"component_start_failed",
		"component_stop_failed",
		"state_changed",
		"drain_started",
		"drain_completed",
		"drain_timed_out",
	}
	got := map[string]string{
		eventComponentStarting:   eventComponentStarting,
		eventComponentStarted:     eventComponentStarted,
		eventComponentStartFailed: eventComponentStartFailed,
		eventComponentStopFailed:  eventComponentStopFailed,
		eventStateChanged:         eventStateChanged,
		eventDrainStarted:         eventDrainStarted,
		eventDrainCompleted:       eventDrainCompleted,
		eventDrainTimedOut:        eventDrainTimedOut,
	}
	for _, w := range want {
		if got[w] != w {
			t.Fatalf("event name %q not defined or mismatched: got %q", w, got[w])
		}
	}
}
