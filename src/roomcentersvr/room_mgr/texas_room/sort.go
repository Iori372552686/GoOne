package texas_room

import (
	"runtime"
	"sort"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

const (
	minChunkSize = 5000 // 并行排序分片大小（小于该值直接单线程排序）
)

// sortRoomsLocked 按请求的排序类型原地排序房间列表，返回错误码。
//
// 调用约定（重要）：必须在持有 TexasRoomCenterMgr 读锁时调用 ——
// less 比较器会读取 Base.CurPlayerNum/EndTime 等会被并发修改的字段，
// 锁外排序既是数据竞争也会得到撕裂的比较结果。
//
// 比较字段统一取自 Base（收集阶段已过滤 Base==nil 的条目）：
// 不使用 RoomShowInfo 顶层冗余字段 RoomId，规避"上报方漏填顶层字段导致排序失效"。
func sortRoomsLocked(rooms []*g1_protocol.RoomShowInfo, sortType g1_protocol.RoomSortType) g1_protocol.ErrorCode {
	if len(rooms) <= 1 {
		return g1_protocol.ErrorCode_ERR_OK
	}

	var less func(i, j int) bool
	switch sortType {
	case g1_protocol.RoomSortType_SORT_TYPE_NONE:
		return g1_protocol.ErrorCode_ERR_OK // 不排序
	case g1_protocol.RoomSortType_SORT_TYPE_ID:
		less = func(i, j int) bool { return rooms[i].Base.RoomId < rooms[j].Base.RoomId }
	case g1_protocol.RoomSortType_SORT_TYPE_PLAYER:
		less = func(i, j int) bool { return rooms[i].Base.CurPlayerNum > rooms[j].Base.CurPlayerNum }
	case g1_protocol.RoomSortType_SORT_TYPE_TIME:
		less = func(i, j int) bool { return rooms[i].Base.EndTime > rooms[j].Base.EndTime }
	default:
		return g1_protocol.ErrorCode_ERR_ARGV
	}

	parallelSort(rooms, less)
	return g1_protocol.ErrorCode_ERR_OK
}

// parallelSort 对 rooms 进行排序（大数据量时并行分片排序 + 串行多路归并）。
// 说明：
//   - less(i,j) 中的 i、j 始终是 rooms 的全局下标（与调用方约定保持一致）
//   - 小数据量直接使用 sort.Slice
//   - 大数据量时：构造索引切片并行排序 + 串行多路归并，最后按索引重排 rooms
//   - 不保证稳定性（与 sort.Slice 一致）
func parallelSort(rooms []*g1_protocol.RoomShowInfo, less func(i, j int) bool) {
	n := len(rooms)
	if n <= 1 {
		return
	}

	// 小数据量直接单线程排序，避免并发开销
	if n <= minChunkSize {
		sort.Slice(rooms, less)
		return
	}

	// 构造索引切片，所有比较都通过全局下标调用 less，避免子切片下标错乱
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}

	// 计算分片数量：不超过 CPU 数，也不超过元素数量 / minChunkSize
	maxWorkers := runtime.GOMAXPROCS(0)
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	chunkCnt := (n + minChunkSize - 1) / minChunkSize
	if chunkCnt > maxWorkers {
		chunkCnt = maxWorkers
	}
	if chunkCnt <= 1 {
		// 退化到单线程索引排序
		sort.Slice(idx, func(i, j int) bool { return less(idx[i], idx[j]) })
		reorderRoomsByIndex(rooms, idx)
		return
	}

	chunkSize := (n + chunkCnt - 1) / chunkCnt

	// 并行排序每个索引分片，分片之间互不重叠，无数据竞争
	type job struct{ start, end int }
	jobs := make(chan job, chunkCnt)
	done := make(chan struct{}, chunkCnt)

	worker := func() {
		for j := range jobs {
			start, end := j.start, j.end
			// 在局部视图上排序，但比较时用全局下标 idx[start+i], idx[start+j]
			sort.Slice(idx[start:end], func(i, j int) bool {
				return less(idx[start+i], idx[start+j])
			})
			done <- struct{}{}
		}
	}

	// 启动 worker
	for w := 0; w < chunkCnt; w++ {
		go worker()
	}

	// 派发任务
	actualChunks := 0
	for start := 0; start < n; start += chunkSize {
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}
		actualChunks++
		jobs <- job{start: start, end: end}
	}
	close(jobs)

	// 等待所有分片排序完成
	for i := 0; i < actualChunks; i++ {
		<-done
	}

	// 将每个已排序分片视为一段有序索引序列，进行串行多路归并
	slices := make([][]int, 0, actualChunks)
	for start := 0; start < n; start += chunkSize {
		end := start + chunkSize
		if end > n {
			end = n
		}
		if start >= end {
			continue
		}
		slices = append(slices, idx[start:end])
	}

	for len(slices) > 1 {
		next := make([][]int, 0, (len(slices)+1)/2)
		for i := 0; i < len(slices); i += 2 {
			if i+1 >= len(slices) {
				next = append(next, slices[i])
				break
			}
			merged := mergeIndexSlices(slices[i], slices[i+1], less)
			next = append(next, merged)
		}
		slices = next
	}

	finalIdx := slices[0]
	reorderRoomsByIndex(rooms, finalIdx)
}

// mergeIndexSlices 合并两个有序索引切片，less 使用全局下标
func mergeIndexSlices(a, b []int, less func(i, j int) bool) []int {
	result := make([]int, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if less(a[i], b[j]) {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}
	if i < len(a) {
		result = append(result, a[i:]...)
	}
	if j < len(b) {
		result = append(result, b[j:]...)
	}
	return result
}

// reorderRoomsByIndex 根据排好序的索引切片重排 rooms
func reorderRoomsByIndex(rooms []*g1_protocol.RoomShowInfo, idx []int) {
	if len(rooms) != len(idx) {
		// 尽早发现错误使用
		panic("reorderRoomsByIndex: length mismatch")
	}
	if len(rooms) <= 1 {
		return
	}

	tmp := make([]*g1_protocol.RoomShowInfo, len(rooms))
	for i, id := range idx {
		tmp[i] = rooms[id]
	}
	copy(rooms, tmp)
}
