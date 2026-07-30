package kubernetes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
	"k8s.io/client-go/util/homedir"
)

func TestSource(t *testing.T) {
	if !itest.Enabled() {
		t.Skip("set GOONE_INTEGRATION=1 to run integration tests")
	}
	home := homedir.HomeDir()
	if _, err := os.Stat(filepath.Join(home, ".kube", "config")); err != nil {
		t.Skipf("kube config unavailable, skipping integration test: %v", err)
	}
	s := NewSource(
		Namespace("mesh"),
		LabelSelector(""),
		KubeConfig(filepath.Join(home, ".kube", "config")),
	)
	kvs, err := s.Load()
	if err != nil {
		t.Error(err)
	}
	for _, v := range kvs {
		t.Log(v)
	}
}
