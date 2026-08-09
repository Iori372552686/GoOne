package logic

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"math/rand"
	"sort"
	"testing"
	"time"
)

func newRoom(id uint64, playerNum uint32, endTime int64) *g1_protocol.RoomShowInfo {
	return &g1_protocol.RoomShowInfo{
		RoomId: id,
		Base: &g1_protocol.RoomBaseInfo{
			CurPlayerNum: playerNum,
			EndTime:      endTime,
		},
	}
}

func cloneRooms(src []*g1_protocol.RoomShowInfo) []*g1_protocol.RoomShowInfo {
	dst := make([]*g1_protocol.RoomShowInfo, len(src))
	copy(dst, src)
	return dst
}

func extractIDs(rooms []*g1_protocol.RoomShowInfo) []uint64 {
	ids := make([]uint64, len(rooms))
	for i, r := range rooms {
		if r != nil {
			ids[i] = r.RoomId
		}
	}
	return ids
}

func TestParallelSort_Empty(t *testing.T) {
	rooms := []*g1_protocol.RoomShowInfo{}
	parallelSort(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })
	if len(rooms) != 0 {
		t.Fatalf("expected len 0, got %d", len(rooms))
	}
}

func TestParallelSort_Single(t *testing.T) {
	rooms := []*g1_protocol.RoomShowInfo{newRoom(1, 10, 100)}
	idsBefore := extractIDs(rooms)
	parallelSort(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })
	idsAfter := extractIDs(rooms)
	if len(rooms) != 1 || idsBefore[0] != idsAfter[0] {
		t.Fatalf("single element should remain unchanged")
	}
}

func TestParallelSort_RoomIdAscending(t *testing.T) {
	rooms := []*g1_protocol.RoomShowInfo{
		newRoom(5, 1, 10),
		newRoom(3, 2, 20),
		newRoom(4, 3, 30),
		newRoom(1, 4, 40),
	}
	parallelSort(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })
	if !sort.SliceIsSorted(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId }) {
		t.Fatalf("rooms not sorted by RoomId ascending: %+v", extractIDs(rooms))
	}
}

func TestParallelSort_ReverseOrder(t *testing.T) {
	rooms := []*g1_protocol.RoomShowInfo{
		newRoom(5, 1, 10),
		newRoom(4, 2, 20),
		newRoom(3, 3, 30),
		newRoom(2, 4, 40),
		newRoom(1, 5, 50),
	}
	parallelSort(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })
	if !sort.SliceIsSorted(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId }) {
		t.Fatalf("rooms not sorted by RoomId ascending: %+v", extractIDs(rooms))
	}
}

func TestParallelSort_LargeRandom(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	n := minChunkSize*2 + 1000
	rooms := make([]*g1_protocol.RoomShowInfo, n)
	for i := 0; i < n; i++ {
		id := uint64(rand.Int63())
		rooms[i] = newRoom(id, uint32(rand.Intn(1000)), rand.Int63())
	}

	// 备份原始 ID 集合
	idsBefore := extractIDs(rooms)
	idCount := make(map[uint64]int, len(idsBefore))
	for _, id := range idsBefore {
		idCount[id]++
	}

	parallelSort(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId })

	if !sort.SliceIsSorted(rooms, func(i, j int) bool { return rooms[i].RoomId < rooms[j].RoomId }) {
		t.Fatalf("rooms not sorted by RoomId ascending")
	}

	// 校验元素未丢失也未新增
	for _, r := range rooms {
		idCount[r.RoomId]--
	}
	for id, c := range idCount {
		if c != 0 {
			t.Fatalf("id %d count mismatch after sort, diff=%d", id, c)
		}
	}
}

func TestParallelSort_ByPlayerNumDesc(t *testing.T) {
	rooms := []*g1_protocol.RoomShowInfo{
		newRoom(1, 10, 100),
		newRoom(2, 30, 100),
		newRoom(3, 20, 100),
	}
	parallelSort(rooms, func(i, j int) bool { return rooms[i].Base.CurPlayerNum > rooms[j].Base.CurPlayerNum })
	if !sort.SliceIsSorted(rooms, func(i, j int) bool { return rooms[i].Base.CurPlayerNum > rooms[j].Base.CurPlayerNum }) {
		t.Fatalf("rooms not sorted by CurPlayerNum desc")
	}
}

func TestParallelSort_ByEndTimeDesc(t *testing.T) {
	rooms := []*g1_protocol.RoomShowInfo{
		newRoom(1, 10, 100),
		newRoom(2, 10, 300),
		newRoom(3, 10, 200),
	}
	parallelSort(rooms, func(i, j int) bool { return rooms[i].Base.EndTime > rooms[j].Base.EndTime })
	if !sort.SliceIsSorted(rooms, func(i, j int) bool { return rooms[i].Base.EndTime > rooms[j].Base.EndTime }) {
		t.Fatalf("rooms not sorted by EndTime desc")
	}
}

func BenchmarkParallelSort_Large(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	n := minChunkSize*3 + 2000
	rooms := make([]*g1_protocol.RoomShowInfo, n)
	for i := 0; i < n; i++ {
		id := uint64(rand.Int63())
		rooms[i] = newRoom(id, uint32(rand.Intn(1000)), rand.Int63())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone := cloneRooms(rooms)
		parallelSort(clone, func(i, j int) bool { return clone[i].RoomId < clone[j].RoomId })
	}
}
