package logic

import (
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"runtime"
	"sort"
	"sync"
	"unsafe"
)

const (
	minChunkSize = 5000 // 分片大小
	mergeWorkers = 4    // 归并并发数
)

// parallelSort 实现分片排序+多路归并 ,可能有bug，需要压力测试 :),备用
func parallelSort(rooms []*g1_protocol.RoomShowInfo, less func(i, j int) bool) {
	if len(rooms) <= minChunkSize {
		sort.Slice(rooms, less)
		return
	}

	chunkCnt := runtime.GOMAXPROCS(runtime.NumCPU())
	chunkSize := (len(rooms) + chunkCnt - 1) / chunkCnt
	chunks := splitAndSort(rooms, chunkSize, less)
	final := mergeChunks(chunks, less, mergeWorkers)

	// 结果复制回原切片
	copy(rooms, final)
}

// splitAndSort 将切片分块并并行排序
func splitAndSort(rooms []*g1_protocol.RoomShowInfo, chunkSize int, less func(i, j int) bool) [][]*g1_protocol.RoomShowInfo {
	var wg sync.WaitGroup
	chunks := make([][]*g1_protocol.RoomShowInfo, 0, (len(rooms)+chunkSize-1)/chunkSize)
	lock := sync.Mutex{}

	for i := 0; i < len(rooms); i += chunkSize {
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			chunk := rooms[start:end]
			sort.Slice(chunk, less)

			lock.Lock()
			chunks = append(chunks, chunk)
			lock.Unlock()
		}(i, min(i+chunkSize, len(rooms)))
	}
	wg.Wait()
	return chunks
}

// mergeChunks 并行多路归并排序后的分片
func mergeChunks(chunks [][]*g1_protocol.RoomShowInfo, less func(i, j int) bool, workers int) []*g1_protocol.RoomShowInfo {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) == 1 {
		return chunks[0]
	}

	// 工作池模式控制并发度
	workChan := make(chan [][]*g1_protocol.RoomShowInfo, len(chunks)/2)
	resultChan := make(chan []*g1_protocol.RoomShowInfo, len(chunks)/2)
	var wg sync.WaitGroup

	// 启动归并worker
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pair := range workChan {
				merged := mergeTwo(pair[0], pair[1], less)
				resultChan <- merged
			}
		}()
	}

	// 分发任务
	go func() {
		for i := 0; i < len(chunks); i += 2 {
			if i+1 < len(chunks) {
				workChan <- [][]*g1_protocol.RoomShowInfo{chunks[i], chunks[i+1]}
			} else {
				resultChan <- chunks[i]
			}
		}
		close(workChan)
	}()

	// 收集结果
	var results [][]*g1_protocol.RoomShowInfo
	go func() {
		for res := range resultChan {
			results = append(results, res)
		}
	}()

	wg.Wait()
	close(resultChan)

	// 递归归并直到完成
	return mergeChunks(results, less, workers)
}

// mergeTwo 合并两个有序切片
func mergeTwo(a, b []*g1_protocol.RoomShowInfo, less func(i, j int) bool) []*g1_protocol.RoomShowInfo {
	result := make([]*g1_protocol.RoomShowInfo, 0, len(a)+len(b))
	i, j := 0, 0

	for i < len(a) && j < len(b) {
		if less(sliceIndex(a, i), sliceIndex(b, j)) {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sliceIndex(slice []*g1_protocol.RoomShowInfo, i int) int {
	if i < 0 || i >= len(slice) {
		panic("index out of range")
	}

	elemSize := unsafe.Sizeof(slice[0])
	base := uintptr(unsafe.Pointer(&slice[0]))
	current := uintptr(unsafe.Pointer(&slice[i]))
	return int((current - base) / elemSize)
}
