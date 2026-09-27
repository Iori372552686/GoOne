package dal

import (
	"context"
	"errors"
	"testing"
)

// fakeCodec：L2 用 "section=value" 字符串分段；L3 用 "blob:<value>" 整包。
type fakeCodec struct{}

func (fakeCodec) MarshalL3(e string) ([]byte, error) { return []byte("blob:" + e), nil }
func (fakeCodec) UnmarshalL3(d []byte) (string, error) {
	if len(d) < 5 || string(d[:5]) != "blob:" {
		return "", errors.New("bad l3 payload")
	}
	return string(d[5:]), nil
}
func (fakeCodec) MarshalL2Fields(e string, sections []string) (map[string][]byte, error) {
	return map[string][]byte{"full": []byte(e)}, nil
}
func (fakeCodec) UnmarshalL2Fields(f map[string][]byte) (string, error) {
	v, ok := f["full"]
	if !ok {
		return "", nil
	}
	return string(v), nil
}

type fakeL2 struct {
	data    map[uint64]map[string][]byte
	loadErr error
	saveErr error
	deleted []uint64
}

func (f *fakeL2) Load(_ context.Context, key uint64) (map[string][]byte, bool, error) {
	if f.loadErr != nil {
		return nil, false, f.loadErr
	}
	fields, ok := f.data[key]
	return fields, ok, nil
}
func (f *fakeL2) Save(_ context.Context, key uint64, fields map[string][]byte) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.data == nil {
		f.data = map[uint64]map[string][]byte{}
	}
	f.data[key] = fields
	return nil
}
func (f *fakeL2) Delete(_ context.Context, key uint64) error {
	f.deleted = append(f.deleted, key)
	delete(f.data, key)
	return nil
}

type fakeL3 struct {
	data     map[uint64][]byte
	saved    int
	saveErr  error
	lastTime int64
	deleted  []uint64
}

func (f *fakeL3) Load(_ context.Context, key uint64) ([]byte, int64, error) {
	d, ok := f.data[key]
	if !ok {
		return nil, 0, nil
	}
	return d, 1, nil
}
func (f *fakeL3) Save(_ context.Context, key uint64, data []byte, ts int64) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.data == nil {
		f.data = map[uint64][]byte{}
	}
	f.data[key] = data
	f.saved++
	f.lastTime = ts
	return nil
}
func (f *fakeL3) Delete(_ context.Context, key uint64) error {
	f.deleted = append(f.deleted, key)
	delete(f.data, key)
	return nil
}

func newFakeStore() (*TieredStore[string], *fakeL2, *fakeL3) {
	l2, l3 := &fakeL2{}, &fakeL3{}
	clock := func() int64 { return 42 }
	return NewTieredStore[string](fakeCodec{}, l2, l3, clock), l2, l3
}

func TestLoadL2Hit(t *testing.T) {
	s, l2, _ := newFakeStore()
	_ = l2.Save(context.Background(), 7, map[string][]byte{"full": []byte("cached")})

	got, found, err := s.Load(context.Background(), 7)
	if err != nil || !found || got != "cached" {
		t.Fatalf("Load = (%q,%v,%v), want (cached,true,nil)", got, found, err)
	}
}

func TestLoadL2MissFallsBackToL3AndBackfills(t *testing.T) {
	s, l2, l3 := newFakeStore()
	l3.data = map[uint64][]byte{7: []byte("blob:cold")}

	got, found, err := s.Load(context.Background(), 7)
	if err != nil || !found || got != "cold" {
		t.Fatalf("Load = (%q,%v,%v), want (cold,true,nil)", got, found, err)
	}
	if _, ok := l2.data[7]; !ok {
		t.Fatal("L2 not backfilled after L3 hit")
	}
}

func TestLoadDoubleMissReturnsNotFound(t *testing.T) {
	s, _, _ := newFakeStore()
	got, found, err := s.Load(context.Background(), 9)
	if err != nil || found || got != "" {
		t.Fatalf("Load = (%q,%v,%v), want (\"\",false,nil)", got, found, err)
	}
}

func TestLoadL2ErrorPropagates(t *testing.T) {
	s, l2, _ := newFakeStore()
	l2.loadErr = errors.New("redis down")
	if _, _, err := s.Load(context.Background(), 7); err == nil {
		t.Fatal("L2 error should propagate（不得静默回源掩盖缓存故障）")
	}
}

func TestSaveL2WritesThrough(t *testing.T) {
	s, l2, _ := newFakeStore()
	if err := s.SaveL2(context.Background(), 7, "entity", nil); err != nil {
		t.Fatal(err)
	}
	if string(l2.data[7]["full"]) != "entity" {
		t.Fatalf("L2 = %v", l2.data[7])
	}
}

func TestSaveL3UsesClockAndCodec(t *testing.T) {
	s, _, l3 := newFakeStore()
	if err := s.SaveL3(context.Background(), 7, "entity"); err != nil {
		t.Fatal(err)
	}
	if string(l3.data[7]) != "blob:entity" {
		t.Fatalf("L3 = %q", l3.data[7])
	}
	if l3.lastTime != 42 {
		t.Fatalf("updateTime = %d, want 42（必须来自注入 clock）", l3.lastTime)
	}
}

func TestSaveL3ErrorPropagatesForRetry(t *testing.T) {
	s, _, l3 := newFakeStore()
	l3.saveErr = errors.New("bus down")
	if err := s.SaveL3(context.Background(), 7, "entity"); err == nil {
		t.Fatal("L3 投递失败必须返回 error 供调用方保留重试标记")
	}
}

func TestDeleteClearsBothTiers(t *testing.T) {
	s, l2, l3 := newFakeStore()
	_ = l2.Save(context.Background(), 7, map[string][]byte{"full": []byte("x")})
	l3.data = map[uint64][]byte{7: []byte("blob:x")}

	if err := s.Delete(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if _, ok := l2.data[7]; ok {
		t.Fatal("L2 not deleted")
	}
	if _, ok := l3.data[7]; ok {
		t.Fatal("L3 not deleted")
	}
}

func TestDefaultClockIsWallClock(t *testing.T) {
	l2, l3 := &fakeL2{}, &fakeL3{}
	s := NewTieredStore[string](fakeCodec{}, l2, l3, nil)
	if s.clock == nil {
		t.Fatal("clock should default to non-nil")
	}
	ts := s.clock()
	if ts < 1700000000000 { // 2023-11 之后，宽松下界防时钟错乱断言过严
		t.Fatalf("default clock = %d, want wall-clock ms", ts)
	}
}
