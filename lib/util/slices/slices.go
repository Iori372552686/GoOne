package util

import (
	"github.com/Iori372552686/GoOne/lib/util/generic"
	"sort"
	"strconv"
)

func Index[E comparable](s []E, v E) int {
	for i, vs := range s {
		if vs == v {
			return i
		}
	}
	return -1
}

// IndexFunc 返回满足f的第一个元素的索引，如果没有满足f的元素则返回-1
func IndexFunc[E any](s []E, f func(E) bool) int {
	for i, v := range s {
		if f(v) {
			return i
		}
	}
	return -1
}

func Contains[E comparable](s []E, v E) bool {
	return Index(s, v) >= 0
}

// ContainsList b中元素是否都在a中
func ContainsList[E comparable](a []E, b []E) bool {
	for _, v := range b {
		if !Contains(a, v) {
			return false
		}
	}
	return true
}

func ContainsFunc[E any](s []E, f func(E) bool) bool {
	return IndexFunc(s, f) >= 0
}

// Remove 从s中移除值为v的第一个元素，remove本身不会改变s
func Remove[E comparable](s []E, v E) ([]E, bool) {
	for i, cv := range s {
		if v == cv {
			return Delete(s, i, i+1), true
		}
	}
	return s, false
}

// RemoveSliceFunc 从s中移除满足f的第一个元素
func RemoveSliceFunc[E any](s []E, f func(E) bool) ([]E, bool) {
	for i, v := range s {
		if f(v) {
			return Delete(s, i, i+1), true
		}
	}
	return s, false
}

// Delete 从s中移除s[i:j]元素，不包含j位置的元素，delete本身不会改变s
func Delete[E any](s []E, i, j int) []E {
	_ = s[i:j]

	return append(s[:i], s[j:]...)
}

func FilterSlice[E any](s []E, f func(E) bool) []E {
	var r []E
	for _, v := range s {
		if f(v) {
			r = append(r, v)
		}
	}
	return r
}

func SliceClone[S ~[]E, E any](s S) S {
	destination := make(S, len(s))
	copy(destination, s)
	return destination
}

func ToMap[K comparable, V any, E any](s []E, f func(int, E) (K, V)) map[K]V {
	m := make(map[K]V, len(s))
	for i, val := range s {
		k, v := f(i, val)
		m[k] = v
	}
	return m
}

func Convert[E1 any, E2 any](s []E1, f func(E1) E2) []E2 {
	r := make([]E2, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}

func IntsToStrings[E generic.Int](s []E) []string {
	return Convert(s, func(e E) string {
		return strconv.Itoa(int(e))
	})
}

type SliceIter[T any] struct {
	slice []T
	index int
}

func (s *SliceIter[T]) Next() bool {
	if s.index >= len(s.slice) {
		return false
	}
	s.index++
	return true
}

func (s *SliceIter[T]) Value() T {
	return s.slice[s.index-1]
}

func NewSliceIter[T any](slice []T) generic.Iter[T] {
	return &SliceIter[T]{slice: slice}
}

func MinIntSlice[T generic.Int](a []T) T {
	if len(a) == 0 {
		return 0
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i] < a[j]
	})
	return a[0]
}

func MaxIntSlice[T generic.Int](a []T) T {
	if len(a) == 0 {
		return 0
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i] < a[j]
	})
	return a[len(a)-1]
}

// 切片头部插入数据，并确保切片长度不超过slen条
func InsertAtHead[T any](slice []T, value T, slen int) []T {
	slice = append([]T{value}, slice...)

	if len(slice) > slen {
		slice = slice[:slen]
	}

	return slice
}

// 切片尾部插入数据，并确保切片长度不超过slen条
func InsertAtTail[T any](slice []T, value T, slen int) []T {
	slice = append(slice, value)

	// 长度超限时，丢弃头部最老的数据
	if len(slice) > slen {
		slice = slice[len(slice)-slen:]
	}

	return slice
}
