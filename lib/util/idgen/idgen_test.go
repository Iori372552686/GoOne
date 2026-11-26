package idgen

import "testing"

func TestGenID(t *testing.T) {
	// Use a fixed machine ID rather than the test runner's network interfaces.
	g := &TIDGen{idGen: newSonyFlake(7)}
	first, err := g.GenID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.GenID()
	if err != nil {
		t.Fatal(err)
	}
	if first == 0 || second <= first {
		t.Fatalf("expected two nonzero, increasing IDs; got %d and %d", first, second)
	}
}
