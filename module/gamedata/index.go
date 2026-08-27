package gamedata

// 本文件定义复合键（多字段主键）的 map key 容器类型，供 gamedata/repository 下
// 自动生成的配置查询代码使用。这些类型是配置访问层的稳定基础设施，手写一次永久生效，
// 不随 cfgtool 重新生成（区别于随 proto 发布的 index.gen.go 旧机制）。
//
// 命名约定：字段名 T{N}, T{N-1}, ..., T{1}（高位在前），与 cfgtool 生成代码中
// 的命名字段初始化（KeyStructInit）对应：List[0] -> T{N}, List[1] -> T{N-1}, ...
//
// 用法（以 2 维复合键 RoomStage+CoinType 为例）：
//
//	map[gamedata.Index2[int32, int32]]*TexasConfig
//	key := gamedata.Index2[int32, int32]{T2: roomStage, T1: coinType}

// Index2 是 2 字段复合键，可作为 map key（所有字段可比较即可）。
type Index2[T2, T1 any] struct {
	T2 T2
	T1 T1
}

// Index3 是 3 字段复合键。
type Index3[T3, T2, T1 any] struct {
	T3 T3
	T2 T2
	T1 T1
}

// Index4 是 4 字段复合键（游戏配置极少超过 4 维；如需更多可按同模式扩展）。
type Index4[T4, T3, T2, T1 any] struct {
	T4 T4
	T3 T3
	T2 T2
	T1 T1
}
