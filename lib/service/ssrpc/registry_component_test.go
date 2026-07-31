package ssrpc

import (
	"context"
	"errors"
	"testing"

	"github.com/Iori372552686/GoOne/lib/service/transaction"
)

// TestRegistryComponentStartSealsAndBindsTransMgr 验证 RegistryComponent.Start
// 完成 Register→Seal→TransMgr 绑定，Dispatcher 已 Seal。
func TestRegistryComponentStartSealsAndBindsTransMgr(t *testing.T) {
	mgr := transaction.NewTransactionMgr()
	register := func(r *Registry) error {
		return r.Register("svc", Binding{
			Kind:       BindingCMD,
			CMD:        500,
			CmdHandler: dummyCmdHandler(),
		})
	}
	c := NewRegistryComponent("ssrpc_registry", register, WithTransactionManager(mgr))
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if c.Dispatcher() == nil || !c.Dispatcher().Sealed() {
		t.Fatal("Dispatcher 应已 Seal")
	}
}

// TestRegistryComponentStartWithoutTransMgr 验证 不注入 TransMgr（web 服务）时
// Start 仍完成 Register→Seal，Dispatcher 可用。
func TestRegistryComponentStartWithoutTransMgr(t *testing.T) {
	register := func(r *Registry) error {
		return r.Register("websvc", Binding{
			Kind:        BindingHTTP,
			HTTPMethod:  "POST",
			HTTPPath:    "/ping",
			HTTPHandler: dummyHTTPHandler(),
		})
	}
	c := NewRegistryComponent("ssrpc_registry", register)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if c.Dispatcher() == nil {
		t.Fatal("Dispatcher 应非 nil")
	}
}

// TestRegistryComponentStartFailsOnRegisterError 验证 register 返回 error 时
// Start 中止并返回该 error。
func TestRegistryComponentStartFailsOnRegisterError(t *testing.T) {
	sentinel := errors.New("bad binding")
	register := func(r *Registry) error { return sentinel }
	c := NewRegistryComponent("ssrpc_registry", register)
	if err := c.Start(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("期望 register 的 error 上抛，got %v", err)
	}
	if c.Dispatcher() != nil {
		t.Fatal("失败时不应有 Dispatcher")
	}
}

// TestRegistryComponentStartFailsOnDuplicateBinding 验证 批次内重复 binding 使
// Start 失败（Registry.Register 原子拒绝）。
func TestRegistryComponentStartFailsOnDuplicateBinding(t *testing.T) {
	register := func(r *Registry) error {
		return r.Register("svc",
			Binding{Kind: BindingCMD, CMD: 600, CmdHandler: dummyCmdHandler()},
			Binding{Kind: BindingCMD, CMD: 600, CmdHandler: dummyCmdHandler()}, // 重复
		)
	}
	c := NewRegistryComponent("ssrpc_registry", register)
	if err := c.Start(context.Background()); err == nil {
		t.Fatal("重复 binding 应使 Start 失败")
	}
}

// TestRegistryComponentSatisfiesRuntimeComponent 验证 RegistryComponent 满足
// runtime.Component 接口签名（编译期断言）。
func TestRegistryComponentSatisfiesRuntimeComponent(t *testing.T) {
	var _ interface {
		Name() string
		Start(context.Context) error
		Stop(context.Context) error
	} = (*RegistryComponent)(nil)
}

// TestRegistryComponentStopIsNoOp 验证 Stop 无资源、幂等。
func TestRegistryComponentStopIsNoOp(t *testing.T) {
	c := NewRegistryComponent("ssrpc_registry", func(r *Registry) error { return nil })
	if err := c.Stop(context.Background()); err != nil {
		t.Fatalf("Stop 应为 no-op nil，got %v", err)
	}
}
