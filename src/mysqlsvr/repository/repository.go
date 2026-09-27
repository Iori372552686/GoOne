package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Iori372552686/GoOne/lib/db/shard"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"
)

var ErrStaleUpdate = errors.New("stale database update")

type DBProvider interface {
	GetDB(name ...string) (*gorm.DB, error)
	Transaction(ctx context.Context, name string, fn func(*gorm.DB) error) error
}

type Store interface {
	UpdateRole(context.Context, uint64, string) error
	SearchRole(context.Context, string) (uint64, error)
	UpsertRoleData(context.Context, uint64, []byte, int64) error
	LoadRoleData(context.Context, uint64) (*g1_protocol.MysqlRoleData, error)
}

type Repository struct {
	db       DBProvider
	roleRule *shard.Rule
}

// roleDataTableBase role_data 逻辑表名（TableShards<=1 时即物理表名）。
const roleDataTableBase = "role_data"

func New(db DBProvider) *Repository {
	return NewWithShard(db, &shard.Rule{Name: "role_data", TableBase: roleDataTableBase})
}

// NewWithShard 构造带分表规则的仓储。rule 通常来自 mysqlsvr.capacity.role_table_shards。
func NewWithShard(db DBProvider, roleRule *shard.Rule) *Repository {
	return &Repository{db: db, roleRule: roleRule}
}

// roleDataTable 解析 uid 对应的物理表名与 ORM 实例名。
func (r *Repository) roleDataTable(uid uint64) (string, string, error) {
	target, err := r.roleRule.Resolve(shard.RouteParams{Uid: uid})
	if err != nil {
		return "", "", fmt.Errorf("resolve role_data route: %w", err)
	}
	return target.Table, target.Instance, nil
}

func (r *Repository) UpdateRole(ctx context.Context, uid uint64, name string) error {
	return r.db.Transaction(ctx, "default", func(tx *gorm.DB) error {
		writeDB := tx.Clauses(dbresolver.Write)
		current := new(g1_protocol.MysqlRoleInfo)
		err := writeDB.Table("role_info").Clauses(clause.Locking{Strength: "UPDATE"}).Where("uid = ?", uid).Take(current).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return writeDB.Table("role_info").Create(&g1_protocol.MysqlRoleInfo{Uid: uid, Name: name}).Error
		case err != nil:
			return err
		default:
			return writeDB.Table("role_info").Where("uid = ?", uid).Update("name", name).Error
		}
	})
}

func (r *Repository) SearchRole(ctx context.Context, name string) (uint64, error) {
	db, err := r.db.GetDB()
	if err != nil {
		return 0, err
	}
	var row struct{ Uid uint64 }
	err = db.WithContext(ctx).Clauses(dbresolver.Read).Table("role_info").Select("uid").Where("name = ?", name).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return row.Uid, err
}

// UpsertRoleData 保存角色全量快照（L3 持久层）。
// data 为 RoleInfo 整包序列化（ConnSvrInfo 由调用方清空），update_time 必须是
// 服务器时钟 ms——不得使用带时区偏移的 Role.Now()，否则跨时区玩家会误判新旧。
//
// 陈旧写守卫：旧行 update_time 严格大于新值时拒绝（相等放行，允许同 ms 重写）。
// 异步写回天然可能乱序，此守卫是兜底而非常态。
func (r *Repository) UpsertRoleData(ctx context.Context, uid uint64, data []byte, updateTime int64) error {
	if uid == 0 {
		return errors.New("role data uid is required")
	}
	if len(data) == 0 {
		return errors.New("role data payload is empty")
	}
	table, instance, err := r.roleDataTable(uid)
	if err != nil {
		return err
	}
	return r.db.Transaction(ctx, instance, func(tx *gorm.DB) error {
		// 注意：gorm 链式方法会累积进同一 statement（clone==0 时原地修改），
		// Take 的 WHERE/LIMIT 会泄漏进后续 UPDATE。每条语句必须经
		// Session(NewDB) 取干净实例。
		take := tx.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Write)
		old := new(g1_protocol.MysqlRoleData)
		err := take.Table(table).Clauses(clause.Locking{Strength: "UPDATE"}).Where("uid = ?", uid).Take(old).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			insert := tx.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Write)
			return insert.Table(table).Create(&g1_protocol.MysqlRoleData{Uid: uid, Data: data, UpdateTime: updateTime}).Error
		case err != nil:
			return err
		default:
			if old.UpdateTime > updateTime {
				return fmt.Errorf("%w: role_data uid=%d old=%d new=%d", ErrStaleUpdate, uid, old.UpdateTime, updateTime)
			}
			update := tx.Session(&gorm.Session{NewDB: true}).Clauses(dbresolver.Write)
			return update.Table(table).Where("uid = ?", uid).
				Updates(map[string]interface{}{"data": data, "update_time": updateTime}).Error
		}
	})
}

// LoadRoleData 按 uid 读取角色全量快照；不存在返回 (nil, nil)。
//
// 刻意走主库（dbresolver.Write）而非读从库：L2 miss 后的回源读若命中滞后副本，
// 会用旧快照重建角色造成数据回退——正确性优先于读分担。此表读频率仅为冷登录，
// 主库压力可忽略。
func (r *Repository) LoadRoleData(ctx context.Context, uid uint64) (*g1_protocol.MysqlRoleData, error) {
	if uid == 0 {
		return nil, errors.New("role data uid is required")
	}
	table, instance, err := r.roleDataTable(uid)
	if err != nil {
		return nil, err
	}
	db, err := r.db.GetDB(instance)
	if err != nil {
		return nil, err
	}
	item := new(g1_protocol.MysqlRoleData)
	err = db.WithContext(ctx).Clauses(dbresolver.Write).Table(table).Where("uid = ?", uid).Take(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

var _ Store = (*Repository)(nil)
