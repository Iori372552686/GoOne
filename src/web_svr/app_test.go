package websvr

import (
	"github.com/Iori372552686/GoOne/common/gconf"
	"github.com/Iori372552686/GoOne/lib/service/bootstrap"
	"github.com/Iori372552686/GoOne/lib/web/web_gin"
	"google.golang.org/grpc"
	"net/http"
	"strings"
	"testing"
)

func TestBuildWebComponentStatuses(t *testing.T) {
	statuses := buildWebComponentStatuses(
		2,
		1,
		3,
		&http.Server{Addr: "127.0.0.1:8080"},
		web_gin.Config{IP: "127.0.0.1", Port: 8080},
		grpc.NewServer(),
		gconf.GRPCServerConfig{Enabled: true, IP: "127.0.0.1", Port: 9090},
	)
	assertWebComponentStatus(t, statuses, "websvr.redis", "ready", true)
	assertWebComponentStatus(t, statuses, "websvr.http_sign", "ready", true)
	assertWebComponentStatus(t, statuses, "websvr.rest_api", "ready", true)
	assertWebComponentStatus(t, statuses, "websvr.http_server", "ready", true)
	assertWebComponentStatus(t, statuses, "websvr.grpc_server", "ready", true)
	if !strings.Contains(findWebComponentStatus(t, statuses, "websvr.http_server").Message, "127.0.0.1:8080") {
		t.Fatalf("expected http server message to include listen addr")
	}
}
func TestBuildWebComponentStatusesPendingAndDisabled(t *testing.T) {
	statuses := buildWebComponentStatuses(
		0,
		0,
		0,
		nil,
		web_gin.Config{IP: "127.0.0.1", Port: 8080},
		nil,
		gconf.GRPCServerConfig{},
	)
	assertWebComponentStatus(t, statuses, "websvr.redis", "pending", false)
	assertWebComponentStatus(t, statuses, "websvr.http_sign", "pending", false)
	assertWebComponentStatus(t, statuses, "websvr.rest_api", "pending", false)
	assertWebComponentStatus(t, statuses, "websvr.http_server", "pending", false)
	assertWebComponentStatus(t, statuses, "websvr.grpc_server", "skipped", true)
}
func findWebComponentStatus(t *testing.T, statuses []bootstrap.ComponentStatus, name string) bootstrap.ComponentStatus {
	t.Helper()
	for _, status := range statuses {
		if status.Name == name {
			return status
		}
	}
	t.Fatalf("component %s not found", name)
	return bootstrap.ComponentStatus{}
}
func assertWebComponentStatus(t *testing.T, statuses []bootstrap.ComponentStatus, name, state string, ready bool) {
	t.Helper()
	status := findWebComponentStatus(t, statuses, name)
	if status.State != state || status.Ready != ready {
		t.Fatalf("unexpected component %s status: %+v", name, status)
	}
}
