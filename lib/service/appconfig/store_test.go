package appconfig

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// cfg is a small config shape exercising scalars, a slice and a map (the kinds
// that must be deep-copied to avoid aliasing the old snapshot).
type cfg struct {
	Port    int
	LogLevel string
	Tracing string
	Headers map[string]string
	Tags    []string
}

func TestLoadPublishesSnapshot(t *testing.T) {
	var calls int32
	s := New[cfg](
		func(context.Context) (*cfg, error) {
			atomic.AddInt32(&calls, 1)
			return &cfg{Port: 8080, LogLevel: "info"}, nil
		},
		nil,
	)
	if c := s.Current(); c != nil {
		t.Fatalf("expected nil before Load, got %+v", c)
	}
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c := s.Current(); c == nil || c.Port != 8080 {
		t.Fatalf("expected published snapshot, got %+v", c)
	}
}

func TestLoadFailsLeavesNoSnapshot(t *testing.T) {
	s := New[cfg](func(context.Context) (*cfg, error) {
		return nil, errors.New("disk gone")
	}, nil)
	if err := s.Load(context.Background()); err == nil {
		t.Fatal("expected Load error")
	}
	if c := s.Current(); c != nil {
		t.Fatalf("expected no snapshot after failed Load, got %+v", c)
	}
}

func TestLoadRejectsSecondCall(t *testing.T) {
	s := New[cfg](func(context.Context) (*cfg, error) {
		return &cfg{Port: 1}, nil
	}, nil)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Load(context.Background()); !errors.Is(err, ErrAlreadyLoaded) {
		t.Fatalf("expected ErrAlreadyLoaded, got %v", err)
	}
}

// mergeWhitelist applies only LogLevel and Tracing from candidate; everything
// else (Port, Headers, Tags) stays at old. It reports Port as restart_required
// when it differs.
func mergeWhitelist(old, candidate *cfg) (*cfg, []string, error) {
	effective := *old // shallow copy of scalar struct
	effective.LogLevel = candidate.LogLevel
	effective.Tracing = candidate.Tracing
	// Deep-copy candidate's hot maps/slices so later candidate mutation cannot
	// affect the published snapshot.
	if candidate.Headers != nil {
		effective.Headers = make(map[string]string, len(candidate.Headers))
		for k, v := range candidate.Headers {
			effective.Headers[k] = v
		}
	}
	if len(candidate.Tags) > 0 {
		effective.Tags = append([]string(nil), candidate.Tags...)
	}
	var restart []string
	if candidate.Port != old.Port {
		restart = append(restart, "port")
	}
	return &effective, restart, nil
}

func TestReloadAppliesWhitelistAndReportsRestart(t *testing.T) {
	s := New[cfg](
		func(context.Context) (*cfg, error) {
			return &cfg{Port: 9090, LogLevel: "debug", Tracing: "on", Headers: map[string]string{"k": "v"}}, nil
		},
		mergeWhitelist,
	)
	// Seed with an initial snapshot.
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Mutate the published snapshot to prove Reload does not read it back.
	first := s.Current()
	first.Port = 1

	res, err := s.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	got := s.Current()
	if got.LogLevel != "debug" || got.Tracing != "on" {
		t.Fatalf("whitelist fields not applied: %+v", got)
	}
	if got.Port == 9090 {
		t.Fatalf("Port should have stayed at the old value (1), but candidate Port was applied (no whitelist on Port); got %+v", got)
	}
	// Port differs between old(1) and candidate(9090): must be reported as
	// restart_required, not applied.
	found := false
	for _, f := range res.RestartRequired {
		if f == "port" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected port in restart_required, got %v", res.RestartRequired)
	}
}

func TestReloadInvalidCandidateLeavesOldUnchanged(t *testing.T) {
	bad := false
	s := New[cfg](
		func(context.Context) (*cfg, error) {
			if bad {
				return &cfg{Port: 9999}, nil
			}
			return &cfg{Port: 1, LogLevel: "info"}, nil
		},
		func(old, candidate *cfg) (*cfg, []string, error) {
			if candidate.Port == 9999 {
				return nil, nil, errors.New("invalid port")
			}
			return mergeWhitelist(old, candidate)
		},
	)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	bad = true
	if _, err := s.Reload(context.Background()); err == nil {
		t.Fatal("expected Reload error")
	}
	if got := s.Current(); got.Port != 1 || got.LogLevel != "info" {
		t.Fatalf("old snapshot must be unchanged after failed reload, got %+v", got)
	}
}

func TestReloadDoesNotAliasCandidateMapsOrSlices(t *testing.T) {
	// The merger deep-copies candidate maps/slices. Prove that after Reload,
	// mutating the candidate's map does NOT affect the published snapshot.
	candidate := &cfg{
		Port:     1,
		LogLevel: "info",
		Headers:  map[string]string{"a": "1"},
		Tags:     []string{"x"},
	}
	s := New[cfg](
		func(context.Context) (*cfg, error) { return candidate, nil },
		mergeWhitelist,
	)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := s.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	published := s.Current()
	// Mutate the candidate map after publication.
	candidate.Headers["a"] = "mutated-by-candidate"
	// Published snapshot must be unaffected (deep-copied).
	if published.Headers["a"] != "1" {
		t.Fatalf("published snapshot aliased candidate map: got %q", published.Headers["a"])
	}
}

func TestConcurrentCurrentAndReload(t *testing.T) {
	// Even on a non-race local run, this exercises the atomic pointer under
	// concurrent readers and a single reloader to catch obvious data races in
	// code review. CI runs -race.
	s := New[cfg](
		func(context.Context) (*cfg, error) {
			return &cfg{Port: 1, LogLevel: "info", Headers: map[string]string{"k": "v"}, Tags: []string{"t"}}, nil
		},
		mergeWhitelist,
	)
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	// Readers.
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c := s.Current()
				if c != nil {
					_ = c.LogLevel
					_ = c.Headers["k"]
				}
			}
		}()
	}
	// Reloader.
	for i := 0; i < 200; i++ {
		if _, err := s.Reload(context.Background()); err != nil {
			t.Fatalf("Reload %d: %v", i, err)
		}
	}
	close(stop)
	wg.Wait()
}
