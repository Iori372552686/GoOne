package transaction

import "runtime"

type TransactionMgrConfig struct {
	// MaxTrans 全局在途事务数上限（超限拒绝新请求）。<=0 回退为 1；
	// 生产装配由 bussvc.TransMgrComponent 再回退到 misc.MaxTransNumber。
	MaxTrans int32
	// ShardCount 派发分片数（队列/transID 空间划分；handler 执行始终是每事务独立
	// goroutine，故它不是并发旋钮）。<=0 回退 DefaultShardCount()。
	ShardCount int
	// MaxPendingPerKey 同一串行键（RouterID/Uid）在前一事务执行期间的最大排队请求数，
	// 超出丢包（dropped_pending_full）——按 key 的内存背压，防止热点 uid/房间刷爆
	// 分片内存。<=0 回退 DefaultMaxPendingPerKey。
	MaxPendingPerKey int
}

// DefaultMaxPendingPerKey 是 MaxPendingPerKey 缺省回退值。历史 0 值语义为"同键并发
// 包直接丢弃"（仅在 useSerialKey=false 的旧单分片路径下无害）；多分片统一后 0 会
// 误丢正常同键请求，故 <=0 一律回退到带排队的默认值。
const DefaultMaxPendingPerKey = 100

type TransactionMgrStats struct {
	ShardCount         int
	ActiveTransactions int64
	PendingPackets     int64
	DroppedPackets     int64
}

func DefaultShardCount() int {
	shardCount := runtime.GOMAXPROCS(0)
	if shardCount <= 0 {
		return 1
	}
	if shardCount > 32 {
		return 32
	}
	return shardCount
}

func normalizeConfig(cfg TransactionMgrConfig) TransactionMgrConfig {
	if cfg.MaxTrans <= 0 {
		cfg.MaxTrans = 1
	}
	if cfg.ShardCount <= 0 {
		cfg.ShardCount = DefaultShardCount()
	}
	if cfg.ShardCount <= 0 {
		cfg.ShardCount = 1
	}
	if cfg.MaxPendingPerKey <= 0 {
		cfg.MaxPendingPerKey = DefaultMaxPendingPerKey
	}
	return cfg
}
