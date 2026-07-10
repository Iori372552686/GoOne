package stats

import (
	"testing"
	"time"
)

func TestCollectorProto(t *testing.T) {
	c := NewCollector()
	key := ProtoKey{Cmd: 0x20001}
	c.RecordRequest(key, "login", "LoginReq", 10*time.Millisecond, OutcomeSuccess, 0)
	c.RecordRequest(key, "login", "LoginReq", 20*time.Millisecond, OutcomeSuccess, 0)
	c.RecordRequest(key, "login", "LoginReq", 0, OutcomeTimeout, 0)

	snap := c.Snapshot()
	if snap.TotalRequests != 3 {
		t.Fatalf("expected total requests 3, got %d", snap.TotalRequests)
	}
	if snap.TotalErrors != 1 {
		t.Fatalf("expected total errors 1, got %d", snap.TotalErrors)
	}

	var login *ModuleSnapshot
	for _, m := range snap.Modules {
		if m.Name == "login" {
			login = m
			break
		}
	}
	if login == nil {
		t.Fatal("missing login module")
	}
	if len(login.Protos) != 1 {
		t.Fatalf("expected 1 proto, got %d", len(login.Protos))
	}
	p := login.Protos[0]
	if p.Total != 3 || p.Success != 2 || p.Timeout != 1 {
		t.Fatalf("unexpected proto snapshot: %+v", p)
	}
	if p.Avg <= 0 {
		t.Fatal("expected avg > 0")
	}
}

func TestCollectorLoop(t *testing.T) {
	c := NewCollector()
	c.RecordLoop("room", true, 100*time.Millisecond)
	c.RecordLoop("room", false, 50*time.Millisecond)

	snap := c.Snapshot()
	if snap.TotalLoops != 2 {
		t.Fatalf("expected total loops 2, got %d", snap.TotalLoops)
	}

	var room *ModuleSnapshot
	for _, m := range snap.Modules {
		if m.Name == "room" {
			room = m
			break
		}
	}
	if room == nil {
		t.Fatal("missing room module")
	}
	if room.Loops != 2 || room.LoopsOK != 1 {
		t.Fatalf("unexpected room snapshot: loops=%d ok=%d", room.Loops, room.LoopsOK)
	}
}
