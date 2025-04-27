package generic

// 所有整数类型以及底层类型为整数的类型
type Int interface {
	~int64 | ~uint64 | ~uint32 | ~int32 | ~int | ~uint
}

type Iter[T any] interface {
	Next() bool
	Value() T
}
