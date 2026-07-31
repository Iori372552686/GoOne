package kuberegistry

import (
	"errors"
	"testing"

	"github.com/Iori372552686/GoOne/lib/contrib/registry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"
)

// failingPodLister 让 List 返回 error，用于验证 P1-07：
// sendLatestInstances 在 list 失败时不再 panic。
type failingPodLister struct {
	listerv1.PodListerExpansion
}

func (failingPodLister) List(labels.Selector) ([]*corev1.Pod, error) {
	return nil, errors.New("simulated cache miss / lister failure")
}

func (failingPodLister) Pods(string) listerv1.PodNamespaceLister { return nil }

// TestSendLatestInstancesDoesNotPanicOnListError 验证 P1-07：list 失败时
// sendLatestInstances 记录并跳过，不 panic 杀死进程。
func TestSendLatestInstancesDoesNotPanicOnListError(t *testing.T) {
	s := &Registry{
		podLister: failingPodLister{},
		stopCh:    make(chan struct{}),
	}
	announcement := make(chan []*registry.ServiceInstance, 1)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sendLatestInstances must not panic on list error, got %v", r)
		}
	}()

	// 期望：不 panic、不向 announcement 投送。
	s.sendLatestInstances("svc-a", announcement)
	select {
	case <-announcement:
		t.Fatal("must not push instances on list error")
	default:
	}
}
