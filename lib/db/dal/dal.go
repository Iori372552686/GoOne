// Package dal 提供三层持久化的编排骨架：L2（可过期缓存）→ L3（最终持久层）。
// L1（进程内 runtime 缓存）由调用方自管——游戏服的实体缓存天然与会话/串行域
// 绑定（如 mainsvr 的 RoleMgr），不适合作通用层。
//
// 泛型于实体类型 T；L2/L3 是字节级接口，实现自行决定序列化与传输
// （Redis 直连 / ssrpc 跨服务均可），框架只负责编排：
//
//   - Load：L2 读穿——miss 时回源 L3，命中后把整包转成分段回填 L2；
//   - SaveL2：写穿 L2（增量分段由调用方指定）；
//   - SaveL3：整包写 L3（one-way 尽力投递；L2 是持久近线存储，L3 无需同步 ack）；
//   - Delete：L2 与 L3 一并清除。
//
// 使用方（如 mainsvr/role）通过实现 EntityCodec 把实体的分段/整包表示接入，
// 角色 section-hash 与整包 BLOB 即是本框架的第一个实例。
package dal

import (
	"context"
	"fmt"
	"time"
)

// EntityCodec 实体 ↔ L2 分段表示 / L3 整包表示 的编解码契约。
type EntityCodec[T any] interface {
	// MarshalL3 实体 → L3 整包字节（运行时字段应在此清除，如 ConnSvrInfo）。
	MarshalL3(entity T) ([]byte, error)
	// UnmarshalL3 L3 整包字节 → 实体。
	UnmarshalL3(data []byte) (T, error)
	// MarshalL2Fields 实体 → L2 分段。sections 为空表示全量段。
	MarshalL2Fields(entity T, sections []string) (map[string][]byte, error)
	// UnmarshalL2Fields L2 分段 → 实体（缺段以零值补，由实现处理）。
	UnmarshalL2Fields(fields map[string][]byte) (T, error)
}

// L2Cache 二级缓存后端（如 Redis hash）。TTL 语义由实现内含。
type L2Cache interface {
	// Load 返回 ok=false 表示无数据（触发 L3 回源）。
	Load(ctx context.Context, key uint64) (fields map[string][]byte, ok bool, err error)
	// Save 写分段（含 TTL 刷新），空 map 为 no-op。
	Save(ctx context.Context, key uint64, fields map[string][]byte) error
	// Delete 清除缓存。
	Delete(ctx context.Context, key uint64) error
}

// L3Store 最终持久层后端。实现通常是跨服务调用（如经 ssrpc 到 mysqlsvr）。
type L3Store interface {
	// Load 返回 data 为 nil 表示无数据（冷启动新建实体）。
	Load(ctx context.Context, key uint64) (data []byte, updateTime int64, err error)
	// Save 写整包快照。updateTime 必须单调（框架经 clock 生成）。
	// 语义为尽力投递：返回 error 仅代表投递失败（调用方保留重试标记），
	// 不代表落库失败——乱序由持久层 updateTime 守卫兜底。
	Save(ctx context.Context, key uint64, data []byte, updateTime int64) error
	// Delete 删除持久数据。
	Delete(ctx context.Context, key uint64) error
}

// TieredStore 两层编排（L1 由调用方持有，miss 时调 Load）。
type TieredStore[T any] struct {
	codec EntityCodec[T]
	l2    L2Cache
	l3    L3Store
	// clock 生成 L3 updateTime（服务器时钟 ms）。禁止用带玩家时区偏移的
	// 业务时间——乱序守卫依赖其单调性。
	clock func() int64
}

// NewTieredStore 构造。clock 为 nil 时用 time.Now().UnixMilli()。
func NewTieredStore[T any](codec EntityCodec[T], l2 L2Cache, l3 L3Store, clock func() int64) *TieredStore[T] {
	if clock == nil {
		clock = func() int64 { return time.Now().UnixMilli() }
	}
	return &TieredStore[T]{codec: codec, l2: l2, l3: l3, clock: clock}
}

// Load 读穿：L2 miss → L3 → 回填 L2。双 miss 返回零值与 found=false（新建实体）。
// L3 回源的实体经 codec 反-正变换重建全量 L2 分段（含 TTL 刷新）。
func (s *TieredStore[T]) Load(ctx context.Context, key uint64) (entity T, found bool, err error) {
	fields, ok, err := s.l2.Load(ctx, key)
	if err != nil {
		var zero T
		return zero, false, fmt.Errorf("dal load l2 key=%d: %w", key, err)
	}
	if ok {
		entity, err = s.codec.UnmarshalL2Fields(fields)
		if err != nil {
			var zero T
			return zero, false, fmt.Errorf("dal decode l2 key=%d: %w", key, err)
		}
		return entity, true, nil
	}

	data, _, err := s.l3.Load(ctx, key)
	if err != nil {
		var zero T
		return zero, false, fmt.Errorf("dal load l3 key=%d: %w", key, err)
	}
	if len(data) == 0 {
		var zero T
		return zero, false, nil
	}
	entity, err = s.codec.UnmarshalL3(data)
	if err != nil {
		var zero T
		return zero, false, fmt.Errorf("dal decode l3 key=%d: %w", key, err)
	}

	// 回填 L2：失败仅降级为下次读再回源，不阻断读路径（也不产生部分状态——
	// L2 写失败时整批不生效，由实现保证）。
	backfill, err := s.codec.MarshalL2Fields(entity, nil)
	if err == nil && len(backfill) > 0 {
		_ = s.l2.Save(ctx, key, backfill)
	}
	return entity, true, nil
}

// SaveL2 写穿 L2（sections 为空 = 全量段）。
func (s *TieredStore[T]) SaveL2(ctx context.Context, key uint64, entity T, sections []string) error {
	fields, err := s.codec.MarshalL2Fields(entity, sections)
	if err != nil {
		return fmt.Errorf("dal encode l2 key=%d: %w", key, err)
	}
	if err := s.l2.Save(ctx, key, fields); err != nil {
		return fmt.Errorf("dal save l2 key=%d: %w", key, err)
	}
	return nil
}

// SaveL3 整包写 L3（尽力投递）。
func (s *TieredStore[T]) SaveL3(ctx context.Context, key uint64, entity T) error {
	data, err := s.codec.MarshalL3(entity)
	if err != nil {
		return fmt.Errorf("dal encode l3 key=%d: %w", key, err)
	}
	if err := s.l3.Save(ctx, key, data, s.clock()); err != nil {
		return fmt.Errorf("dal save l3 key=%d: %w", key, err)
	}
	return nil
}

// Delete 同时清 L2 与 L3。
func (s *TieredStore[T]) Delete(ctx context.Context, key uint64) error {
	if err := s.l3.Delete(ctx, key); err != nil {
		return fmt.Errorf("dal delete l3 key=%d: %w", key, err)
	}
	if err := s.l2.Delete(ctx, key); err != nil {
		return fmt.Errorf("dal delete l2 key=%d: %w", key, err)
	}
	return nil
}
