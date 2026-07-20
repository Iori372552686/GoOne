package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRegistryRejectsDuplicateModule(t *testing.T) {
	reg := NewRegistry()
	m1 := ModuleFunc{ModuleName: "bus", OnRegister: func(*Registry) error { return nil }}
	m2 := ModuleFunc{ModuleName: "bus", OnRegister: func(*Registry) error { return nil }}
	if err := reg.RegisterModule(m1); err != nil {
		t.Fatalf("first module: %v", err)
	}
	err := reg.RegisterModule(m2)
	if !errors.Is(err, ErrDuplicateModule) {
		t.Fatalf("expected ErrDuplicateModule, got %v", err)
	}
}

func TestRegistryRejectsDuplicateComponent(t *testing.T) {
	reg := NewRegistry()
	c1 := ComponentFunc{ComponentName: "redis", OnStart: func(context.Context) error { return nil }}
	c2 := ComponentFunc{ComponentName: "redis", OnStart: func(context.Context) error { return nil }}
	if err := reg.RegisterComponent(c1); err != nil {
		t.Fatalf("first component: %v", err)
	}
	err := reg.RegisterComponent(c2)
	if !errors.Is(err, ErrDuplicateComponent) {
		t.Fatalf("expected ErrDuplicateComponent, got %v", err)
	}
}

func TestRegistryRejectsNilComponent(t *testing.T) {
	reg := NewRegistry()
	if err := reg.RegisterComponent(nil); !errors.Is(err, ErrNilComponent) {
		t.Fatalf("expected ErrNilComponent, got %v", err)
	}
}

func TestRegistryRejectsNilModule(t *testing.T) {
	reg := NewRegistry()
	if err := reg.RegisterModule(nil); !errors.Is(err, ErrNilModule) {
		t.Fatalf("expected ErrNilModule, got %v", err)
	}
}

func TestRegistryRejectsRegistrationAfterSeal(t *testing.T) {
	reg := NewRegistry()
	if _, err := reg.Seal(); err != nil {
		t.Fatalf("seal: %v", err)
	}
	if !reg.IsSealed() {
		t.Fatal("expected sealed")
	}
	// Idempotent seal.
	if _, err := reg.Seal(); err != nil {
		t.Fatalf("idempotent seal: %v", err)
	}
	err := reg.RegisterComponent(ComponentFunc{ComponentName: "late", OnStart: func(context.Context) error { return nil }})
	if !errors.Is(err, ErrRegistrySealed) {
		t.Fatalf("expected ErrRegistrySealed, got %v", err)
	}
	err = reg.RegisterModule(ModuleFunc{ModuleName: "late", OnRegister: func(*Registry) error { return nil }})
	if !errors.Is(err, ErrRegistrySealed) {
		t.Fatalf("expected ErrRegistrySealed for module, got %v", err)
	}
}

func TestRegistryPreservesOrder(t *testing.T) {
	reg := NewRegistry()
	register := func(name string) {
		_ = reg.RegisterComponent(ComponentFunc{
			ComponentName: name,
			OnStart: func(context.Context) error { return nil },
		})
	}
	// Register via a module so both module and component ordering is exercised.
	mod := ModuleFunc{ModuleName: "main", OnRegister: func(r *Registry) error {
		register("a")
		register("b")
		register("c")
		return nil
	}}
	if err := reg.RegisterModule(mod); err != nil {
		t.Fatalf("register module: %v", err)
	}
	components, err := reg.Seal()
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if len(components) != 3 {
		t.Fatalf("expected 3 components, got %d", len(components))
	}
	for i, want := range []string{"a", "b", "c"} {
		if components[i].Name() != want {
			t.Fatalf("component %d: want %s, got %s", i, want, components[i].Name())
		}
	}
}

func TestModuleRegisterFailureAbortsAndStartsNothing(t *testing.T) {
	// NewFromModules must return an error and build no App when a module fails.
	broken := ModuleFunc{ModuleName: "bad", OnRegister: func(r *Registry) error {
		if err := r.RegisterComponent(ComponentFunc{ComponentName: "ok", OnStart: func(context.Context) error { return nil }}); err != nil {
			return err
		}
		return errors.New("module exploded")
	}}
	a, err := NewFromModules("svc", []Module{broken})
	if err == nil {
		t.Fatal("expected module failure to propagate")
	}
	if a != nil {
		t.Fatal("expected no App on module failure")
	}
	// The error should carry the module name.
	if err.Error() == "" || !containsStr(err.Error(), "bad") {
		t.Fatalf("expected error to mention module name, got %q", err.Error())
	}
}

func TestRegistryModuleErrorWrapsInner(t *testing.T) {
	inner := errors.New("db unreachable")
	reg := NewRegistry()
	mod := ModuleFunc{ModuleName: "store", OnRegister: func(*Registry) error { return inner }}
	err := reg.RegisterModule(mod)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected wrapped inner error, got %v", err)
	}
}

func TestNewFromModulesStartsComponentsInOrder(t *testing.T) {
	var mu sync.Mutex
	trace := make([]string, 0)
	mkComp := func(name string) Component {
		return ComponentFunc{
			ComponentName: name,
			OnStart: func(context.Context) error {
				mu.Lock()
				trace = append(trace, name+":start")
				mu.Unlock()
				return nil
			},
		}
	}
	mod := ModuleFunc{ModuleName: "m", OnRegister: func(r *Registry) error {
		if err := r.RegisterComponent(mkComp("a")); err != nil {
			return err
		}
		if err := r.RegisterComponent(mkComp("b")); err != nil {
			return err
		}
		return r.RegisterComponent(mkComp("c"))
	}}
	a, err := NewFromModules("svc", []Module{mod})
	if err != nil {
		t.Fatalf("NewFromModules: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := runInBackground(t, a, ctx)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run: %v", err)
	}
	mu.Lock()
	got := append([]string(nil), trace...)
	mu.Unlock()
	want := []string{"a:start", "b:start", "c:start"}
	for i, w := range want {
		if i >= len(got) || got[i] != w {
			t.Fatalf("start order mismatch: want %v, got %v", want, got)
		}
	}
}

func containsStr(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
