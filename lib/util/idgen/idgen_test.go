package idgen

import "testing"

func TestGenID(t *testing.T) {
	g, err := NewIDGen()
	if err != nil {
		t.Fatal(err)
	}
	id, err := g.GenID()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(id)
}
