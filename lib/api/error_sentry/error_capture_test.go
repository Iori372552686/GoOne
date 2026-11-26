package error_sentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
)

func Test_ErrorCapture(t *testing.T) {
	type request struct{ method, path string }
	received := make(chan request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- request{r.Method, r.URL.Path}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	previousConfig, previousClient := sentryCnf, sentry.CurrentHub().Client()
	t.Cleanup(func() {
		sentryCnf = previousConfig
		sentry.CurrentHub().BindClient(previousClient)
	})

	// Only the loopback server receives the event; CI must not send telemetry.
	dsn := "http://test-key@" + strings.TrimPrefix(server.URL, "http://") + "/1"
	if err := InitSentry(dsn, "test-version", "test-service", "test-release", "test-player"); err != nil {
		t.Fatal(err)
	}
	CaptureErrorToSentry(sentry.LevelError, "test-player", "test-user", "test error: %s", "example")
	if !sentry.Flush(5 * time.Second) {
		t.Fatal("timed out waiting for Sentry test event")
	}
	select {
	case got := <-received:
		if got.method != http.MethodPost || got.path != "/api/1/envelope/" {
			t.Fatalf("unexpected Sentry request: %+v", got)
		}
	default:
		t.Fatal("loopback server did not receive a Sentry event")
	}
}
