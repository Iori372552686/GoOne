package bus

import (
	"errors"
	"strings"
	"testing"
)

func TestDriverRegistryRejectsEmptyAndNil(t *testing.T) {
	r := NewDriverRegistry()
	if err := r.Register(Driver{}); !errors.Is(err, ErrEmptyDriverName) {
		t.Fatalf("expected ErrEmptyDriverName, got %v", err)
	}
	if err := r.Register(Driver{Name: "x"}); !errors.Is(err, ErrNilDriverCtor) {
		t.Fatalf("expected ErrNilDriverCtor, got %v", err)
	}
}

func TestDriverRegistryRejectsDuplicate(t *testing.T) {
	r := NewDriverRegistry()
	d := Driver{Name: "rabbitmq", Ctor: func(uint32, MsgHandler, any) (IBus, error) { return nil, nil }}
	if err := r.Register(d); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := r.Register(d); !errors.Is(err, ErrDuplicateDriver) {
		t.Fatalf("expected ErrDuplicateDriver, got %v", err)
	}
}

func TestDriverRegistryRegisterAllAtomic(t *testing.T) {
	r := NewDriverRegistry()
	ctor := func(uint32, MsgHandler, any) (IBus, error) { return nil, nil }
	// Second entry duplicates the first within the batch -> whole batch rejected.
	err := r.RegisterAll(
		Driver{Name: "rabbitmq", Ctor: ctor},
		Driver{Name: "rabbitmq", Ctor: ctor},
	)
	if !errors.Is(err, ErrDuplicateDriver) {
		t.Fatalf("expected ErrDuplicateDriver, got %v", err)
	}
	if len(r.drivers) != 0 {
		t.Fatalf("atomic batch must leave registry empty on failure, got %d", len(r.drivers))
	}
}

func TestDriverRegistryCreateBusRejectsUnlinked(t *testing.T) {
	r := NewDriverRegistry()
	// No drivers registered: requesting rabbitmq must fail with a message that
	// lists available drivers (empty here).
	_, err := r.CreateBus(1, nil, "amqp://guest:guest@localhost:5672")
	if err == nil {
		t.Fatal("expected error for unlinked driver")
	}
	if !strings.Contains(err.Error(), "rabbitmq") {
		t.Fatalf("error should name the missing driver, got %q", err.Error())
	}
}

func TestDriverRegistryCreateBusUsesRegisteredOnly(t *testing.T) {
	r := NewDriverRegistry()
	called := false
	// Register only a fake "rabbitmq"; real rabbitmq driver ctor is not needed
	// because ParseAddr("amqp://...") yields implType "rabbitmq" + string cfg.
	if err := r.Register(Driver{
		Name: "rabbitmq",
		Ctor: func(_ uint32, _ MsgHandler, _ any) (IBus, error) {
			called = true
			return nil, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := r.CreateBus(1, nil, "amqp://guest:guest@localhost:5672"); err != nil {
		t.Fatalf("CreateBus: %v", err)
	}
	if !called {
		t.Fatal("expected registered ctor to be invoked")
	}
}

func TestDriverRegistryNamesSorted(t *testing.T) {
	r := NewDriverRegistry()
	ctor := func(uint32, MsgHandler, any) (IBus, error) { return nil, nil }
	for _, n := range []string{"rocketmq", "rabbitmq", "kafka"} {
		if err := r.Register(Driver{Name: n, Ctor: ctor}); err != nil {
			t.Fatalf("register %s: %v", n, err)
		}
	}
	got := r.Names()
	want := []string{"kafka", "rabbitmq", "rocketmq"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("expected sorted %v, got %v", want, got)
	}
}
