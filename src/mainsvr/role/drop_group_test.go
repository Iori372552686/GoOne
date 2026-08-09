/// 掉落系统纯函数单测（不依赖 Role/配置加载）
package role

import (
	"testing"

	pb "github.com/Iori372552686/g1_common/protocol"
)

func TestProbabilityHit(t *testing.T) {
	if probabilityHit(0) {
		t.Error("rate=0 应永不命中")
	}
	if probabilityHit(-1) {
		t.Error("rate<0 应永不命中")
	}
	if !probabilityHit(10000) {
		t.Error("rate>=10000 应必定命中")
	}
	if !probabilityHit(100000) {
		t.Error("rate>10000 应必定命中")
	}
	// rate=5000 统计频率应接近 0.5
	hit := 0
	n := 10000
	for i := 0; i < n; i++ {
		if probabilityHit(5000) {
			hit++
		}
	}
	freq := float64(hit) / float64(n)
	if freq < 0.45 || freq > 0.55 {
		t.Errorf("rate=5000 频率 %.3f 偏离 0.5 过多", freq)
	}
}

func TestWeightedPickKey(t *testing.T) {
	m := map[int32]int32{1: 7000, 2: 3000}
	cnt := map[int32]int{}
	n := 10000
	for i := 0; i < n; i++ {
		k, ok := weightedPickKey(m)
		if !ok {
			t.Fatal("weightedPickKey 应命中")
		}
		cnt[k]++
	}
	// 7000:3000 → 频率应接近 0.7:0.3
	f1 := float64(cnt[1]) / float64(n)
	f2 := float64(cnt[2]) / float64(n)
	if f1 < 0.65 || f1 > 0.75 {
		t.Errorf("key=1 频率 %.3f 偏离 0.7", f1)
	}
	if f2 < 0.25 || f2 > 0.35 {
		t.Errorf("key=2 频率 %.3f 偏离 0.3", f2)
	}

	// 全零权重应不命中
	if _, ok := weightedPickKey(map[int32]int32{1: 0, 2: 0}); ok {
		t.Error("全零权重应返回 false")
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[int32]int32{30: 1, 10: 2, 20: 3}
	got := sortedKeys(m)
	want := []int32{10, 20, 30}
	if len(got) != len(want) {
		t.Fatalf("长度不符: got=%v want=%v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("第%d个 key: got=%d want=%d", i, got[i], want[i])
		}
	}
}

func TestRootRowAndEnabledSorted(t *testing.T) {
	rows := []*pb.DropGroupConfig{
		{Groupid: 1, Subid: 3, Type: 0, IsBan: 0},
		{Groupid: 1, Subid: 1, Type: 0, IsBan: 0},
		{Groupid: 1, Subid: 2, Type: 0, IsBan: 1}, // 禁用
	}
	root := rootRow(rows)
	if root.Subid != 1 {
		t.Errorf("root 应取 Subid 最小=1, got=%d", root.Subid)
	}
	enabled := enabledRowsSorted(rows)
	if len(enabled) != 2 {
		t.Fatalf("启用行应为2(剔除IsBan=1), got=%d", len(enabled))
	}
	if enabled[0].Subid != 1 || enabled[1].Subid != 3 {
		t.Errorf("启用行应按 Subid 升序: got %d,%d", enabled[0].Subid, enabled[1].Subid)
	}
}

func TestRandomItemEntriesAll(t *testing.T) {
	entries := []*pb.DropItemConfig{
		{DropId: 1, ItemId: 100, Count: 10, DropWay: dropItemWayAll, Probability: 10000},
		{DropId: 1, ItemId: 101, Count: 5, DropWay: dropItemWayAll, Probability: 10000},
	}
	got := randomItemEntries(entries)
	if len(got) != 2 {
		t.Errorf("dropWay=1(全产) 应返回全部2条, got=%d", len(got))
	}
}

func TestRandomItemEntriesRandom1(t *testing.T) {
	entries := []*pb.DropItemConfig{
		{DropId: 1, ItemId: 100, Count: 10, DropWay: dropItemWayRandom1, Probability: 5000},
		{DropId: 1, ItemId: 101, Count: 5, DropWay: dropItemWayRandom1, Probability: 5000},
	}
	got := randomItemEntries(entries)
	if len(got) != 1 {
		t.Errorf("dropWay=2(随机选1) 应返回1条, got=%d", len(got))
	}
}
