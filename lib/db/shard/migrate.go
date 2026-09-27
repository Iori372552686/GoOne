package shard

import (
	"context"
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

// 本文件提供分表的自动建表与一次性迁移（均针对 MySQL）。
//
// 建表经 gorm AutoMigrate：Table 子句会经 migrator.RunWithValue 继承为物理表名
// （gorm v1.31 migrator/migrator.go:65 stmt.Table = m.DB.Statement.Table），
// 因此 db.Table(name).AutoMigrate(model) 建的是 name 而非模型默认命名。

// EnsureTables 确保规则覆盖的物理表存在：
//   - TableShards<=1：只确保基表 {TableBase} 存在；
//   - TableShards>1 ：确保 {TableBase}_0 .. {TableBase}_{N-1} 全部存在。
//
// 幂等，可在每次启动时调用。列结构以 model 的 gorm tag 为准（与 repository
// 的读写列对齐由调用方保证——model 与查询用的是同一个 proto ORM struct）。
func EnsureTables(ctx context.Context, db *gorm.DB, rule *Rule, model interface{}) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("shard ensure tables: nil db")
	}
	targets := []string{rule.TableBase}
	if rule.TableShards > 1 {
		targets = targets[:0]
		for i := 0; i < rule.TableShards; i++ {
			targets = append(targets, physicalTable(rule.TableBase, i))
		}
	}
	for _, name := range targets {
		if err := db.WithContext(ctx).Table(name).AutoMigrate(model); err != nil {
			return fmt.Errorf("shard ensure table %s: %w", name, err)
		}
	}
	return nil
}

// MigrateUnsharded 把未分片基表的存量行迁入分表，全部成功后将基表改名为
// {TableBase}__legacy（保留原数据以备回滚，由 DBA 决定何时删除）。
//
// 语义与边界：
//   - 仅在 TableShards>1 时有意义，<=1 直接返回（无操作）；
//   - 幂等：基表不存在（从未存在或已迁移改名）时直接返回 0, nil；
//   - 前置条件：分表已存在（先调 EnsureTables）；
//   - 逐行 INSERT，不做事务包裹：中途失败保留已迁行，重跑前需人工清理分表
//     中的半程数据（或按 uid UPSERT 去重）。线上行数大时应改为停机/低峰执行；
//   - rows 必须是 *[]T（gorm Find 的目标），keyOf 返回第 i 行的分片键。
func MigrateUnsharded(ctx context.Context, db *gorm.DB, rule *Rule, rows interface{}, keyOf func(i int) uint64) (int, error) {
	if err := rule.Validate(); err != nil {
		return 0, err
	}
	if db == nil {
		return 0, fmt.Errorf("shard migrate: nil db")
	}
	if rule.TableShards <= 1 {
		return 0, nil
	}
	if !db.Migrator().HasTable(rule.TableBase) {
		return 0, nil // 已迁移（基表已改名）或从未存在
	}

	if err := db.WithContext(ctx).Table(rule.TableBase).Find(rows).Error; err != nil {
		return 0, fmt.Errorf("shard migrate: load base rows: %w", err)
	}
	rv := reflect.ValueOf(rows)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, fmt.Errorf("shard migrate: rows must be a non-nil slice pointer")
	}
	slice := rv.Elem()
	if slice.Kind() != reflect.Slice {
		return 0, fmt.Errorf("shard migrate: rows must point to a slice, got %s", slice.Kind())
	}

	for i := 0; i < slice.Len(); i++ {
		uid := keyOf(i)
		target, err := rule.Resolve(RouteParams{Uid: uid})
		if err != nil {
			return i, err
		}
		if err := db.WithContext(ctx).Table(target.Table).Create(slice.Index(i).Addr().Interface()).Error; err != nil {
			return i, fmt.Errorf("shard migrate: insert uid=%d into %s: %w", uid, target.Table, err)
		}
	}

	legacy := LegacyName(rule.TableBase)
	if err := db.Migrator().RenameTable(rule.TableBase, legacy); err != nil {
		return slice.Len(), fmt.Errorf("shard migrate: rename base to %s: %w", legacy, err)
	}
	return slice.Len(), nil
}

// LegacyName 已迁移基表的重命名目标。
func LegacyName(base string) string {
	return base + "__legacy"
}

// physicalTable 第 idx 张分表的物理表名。
func physicalTable(base string, idx int) string {
	return fmt.Sprintf("%s_%d", base, idx)
}
