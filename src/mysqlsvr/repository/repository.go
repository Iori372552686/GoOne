package repository

import (
	"context"
	"errors"
	"fmt"

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
	QueryRoom(context.Context, *g1_protocol.QueryRoomInfoReq) ([]*g1_protocol.MysqlTexasRoomInfo, error)
	QueryPlayer(context.Context, *g1_protocol.QueryPlayerInfoReq) ([]*g1_protocol.MysqlTexasPlayerInfo, error)
	GetGame(context.Context, string) (*g1_protocol.MysqlTexasGameInfo, error)
	SaveRoom(context.Context, *g1_protocol.MysqlTexasRoomInfo) error
	SaveGame(context.Context, *g1_protocol.MysqlTexasGameInfo) error
	InsertPlayer(context.Context, *g1_protocol.MysqlTexasPlayerInfo) error
}

type Repository struct {
	db DBProvider
}

func New(db DBProvider) *Repository { return &Repository{db: db} }

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

func (r *Repository) QueryRoom(ctx context.Context, req *g1_protocol.QueryRoomInfoReq) ([]*g1_protocol.MysqlTexasRoomInfo, error) {
	db, err := r.db.GetDB()
	if err != nil {
		return nil, err
	}
	query := db.WithContext(ctx).Clauses(dbresolver.Read).Where("room_id = ?", req.GetRoomId())
	if req.GetTableId() > 0 {
		query = query.Where("table_id = ?", req.GetTableId())
	}
	if req.GetGameType() > 0 {
		query = query.Where("game_type = ?", req.GetGameType())
	}
	if req.GetRoomStage() > 0 {
		query = query.Where("room_stage = ?", req.GetRoomStage())
	}
	if req.GetBlind() != "" {
		query = query.Where("blind = ?", req.GetBlind())
	}
	if req.GetBeginTime() > 0 {
		query = query.Where("create_time >= ?", req.GetBeginTime())
	}
	if req.GetEndTime() > 0 {
		query = query.Where("finish_time <= ?", req.GetEndTime())
	}
	items := make([]*g1_protocol.MysqlTexasRoomInfo, 0)
	return items, query.Find(&items).Error
}

func (r *Repository) QueryPlayer(ctx context.Context, req *g1_protocol.QueryPlayerInfoReq) ([]*g1_protocol.MysqlTexasPlayerInfo, error) {
	db, err := r.db.GetDB()
	if err != nil {
		return nil, err
	}
	query := db.WithContext(ctx).Clauses(dbresolver.Read).Where("uid = ?", req.GetUid())
	if req.GetTableId() > 0 {
		query = query.Where("table_id = ?", req.GetTableId())
	}
	if req.GetRoomId() > 0 {
		query = query.Where("room_id = ?", req.GetRoomId())
	}
	if req.GetGameType() > 0 {
		query = query.Where("game_type = ?", req.GetGameType())
	}
	if req.GetRoomStage() > 0 {
		query = query.Where("room_stage = ?", req.GetRoomStage())
	}
	if req.GetBlind() != "" {
		query = query.Where("blind = ?", req.GetBlind())
	}
	if req.GetBeginTime() > 0 {
		query = query.Where("begin_time >= ?", req.GetBeginTime())
	}
	if req.GetEndTime() > 0 {
		query = query.Where("end_time <= ?", req.GetEndTime())
	}
	items := make([]*g1_protocol.MysqlTexasPlayerInfo, 0)
	return items, query.Find(&items).Error
}

func (r *Repository) GetGame(ctx context.Context, gameID string) (*g1_protocol.MysqlTexasGameInfo, error) {
	db, err := r.db.GetDB()
	if err != nil {
		return nil, err
	}
	item := new(g1_protocol.MysqlTexasGameInfo)
	err = db.WithContext(ctx).Clauses(dbresolver.Read).Where("game_id = ?", gameID).First(item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return item, err
}

func (r *Repository) SaveRoom(ctx context.Context, item *g1_protocol.MysqlTexasRoomInfo) error {
	if item == nil {
		return errors.New("room info is nil")
	}
	return r.db.Transaction(ctx, "default", func(tx *gorm.DB) error {
		writeDB := tx.Clauses(dbresolver.Write)
		old := new(g1_protocol.MysqlTexasRoomInfo)
		err := writeDB.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("room_id = ? AND table_id = ?", item.RoomId, item.TableId).First(old).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return writeDB.Create(item).Error
		}
		if err != nil {
			return err
		}
		if old.UpdateTime > item.UpdateTime {
			return fmt.Errorf("%w: room_id=%d table_id=%d old=%d new=%d", ErrStaleUpdate, item.RoomId, item.TableId, old.UpdateTime, item.UpdateTime)
		}
		return writeDB.Model(old).Where("id = ?", old.Id).Select("*").Omit("id").Updates(item).Error
	})
}

func (r *Repository) SaveGame(ctx context.Context, item *g1_protocol.MysqlTexasGameInfo) error {
	if item == nil {
		return errors.New("game info is nil")
	}
	return r.db.Transaction(ctx, "default", func(tx *gorm.DB) error {
		writeDB := tx.Clauses(dbresolver.Write)
		old := new(g1_protocol.MysqlTexasGameInfo)
		err := writeDB.Clauses(clause.Locking{Strength: "UPDATE"}).Where("game_id = ?", item.GameId).First(old).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return writeDB.Create(item).Error
		}
		if err != nil {
			return err
		}
		if old.UpdateTime > item.UpdateTime {
			return fmt.Errorf("%w: game_id=%s old=%d new=%d", ErrStaleUpdate, item.GameId, old.UpdateTime, item.UpdateTime)
		}
		return writeDB.Model(old).Where("game_id = ?", old.GameId).Select("*").Omit("game_id").Updates(item).Error
	})
}

func (r *Repository) InsertPlayer(ctx context.Context, item *g1_protocol.MysqlTexasPlayerInfo) error {
	if item == nil {
		return errors.New("player info is nil")
	}
	db, err := r.db.GetDB()
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Clauses(dbresolver.Write).Create(item).Error
}

var _ Store = (*Repository)(nil)
