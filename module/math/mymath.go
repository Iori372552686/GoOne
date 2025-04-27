package math

import "math/rand"

// 返回idx
func WeightedRandomSelect(weightList []int32) int32 {
	totalWeight := 0
	for _, v := range weightList {
		totalWeight += int(v)
	}

	r := rand.Intn(totalWeight)
	tmpWeight := totalWeight
	for i, v := range weightList {
		tmpWeight -= int(v)
		if r >= tmpWeight {
			return int32(i)
		}
	}
	return int32(len(weightList) - 1)
}

// 带权随机出n(>=1)个
// 比较笨的办法
// 一个一个的抽
func WeightedRandomSelectN(weightList []int32, n int32) *[]int32 {
	ret := make([]int32, 0)
	wlen := int32(len(weightList))
	if n <= 0 || wlen <= 0 {
		return nil
	}

	tmpWeight := make([]int32, wlen)
	copy(tmpWeight, weightList)
	for i := int32(0); i < n && i < wlen; i++ {
		idx := WeightedRandomSelect(tmpWeight)
		ret = append(ret, idx)
		tmpWeight[idx] = 0
	}
	return &ret
}

func RandomSelectN(max, n int32) *[]int32 {
	// TODO 比较粗暴，也可以用一个数组然后shuffle
	w := make([]int32, max)
	for i := int32(0); i < max; i++ {
		w[i] = 1
	}
	return WeightedRandomSelectN(w, n)
}
