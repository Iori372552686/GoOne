package error_capture

import (
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

func Test_ErrorCapture(t *testing.T) {
	str := "测"
	userId := "0526_90001"
	userName := "Name_0526_90001"
	id := 052601
	errMsg := "cs"

	// InitBugsnag(BUGSNAG_VERSION_READY, "game_ready", "3")
	// Error(userId, userName, str, id, errMsg)

	InitSentry(
		"https://1fb15babfd3b4705b371b2819db269c7:9aaef6b1d5bd4cf5ac02e45b461c8184@sentry.io/1859718",

		"test_version_0526",
		"test_0526",
		"test_release_0526",
		userId,
	)

	CaptureErrorToSentry(sentry.LevelError, userId, userName, str, id, errMsg)
	time.Sleep(5 * time.Second)
}
