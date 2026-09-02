package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type testDBProvider struct{ db *gorm.DB }

func (p testDBProvider) GetDB(...string) (*gorm.DB, error) { return p.db, nil }

func (p testDBProvider) Transaction(ctx context.Context, _ string, fn func(*gorm.DB) error) error {
	return p.db.WithContext(ctx).Transaction(fn)
}

func newRepositoryTestDB(t *testing.T) (*Repository, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	pool, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: pool, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing: true,
		NamingStrategy:       schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(testDBProvider{db: db}), mock, pool
}

func TestSaveRoomRejectsStaleUpdate(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `mysql_texas_room_info` WHERE room_id = ? AND table_id = ? ORDER BY `mysql_texas_room_info`.`id` LIMIT ? FOR UPDATE")).
		WithArgs(uint64(10), uint64(20), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "room_id", "table_id", "update_time"}).AddRow(1, 10, 20, 200))
	mock.ExpectRollback()

	err := repo.SaveRoom(context.Background(), &g1_protocol.MysqlTexasRoomInfo{
		RoomId: 10, TableId: 20, UpdateTime: 100,
	})
	if !errors.Is(err, ErrStaleUpdate) {
		t.Fatalf("SaveRoom() error = %v, want ErrStaleUpdate", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestQueryRoomAppliesOptionalFilters(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectQuery("SELECT .* FROM `mysql_texas_room_info` WHERE room_id = \\? AND table_id = \\? AND game_type = \\? AND create_time >= \\? AND finish_time <= \\?").
		WithArgs(uint64(10), uint64(20), g1_protocol.GameTypeId(1), int64(100), int64(200)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := repo.QueryRoom(context.Background(), &g1_protocol.QueryRoomInfoReq{
		RoomId: 10, TableId: 20, GameType: 1, BeginTime: 100, EndTime: 200,
	})
	if err != nil {
		t.Fatalf("QueryRoom() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetGameMapsRecordNotFoundToNil(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectQuery("SELECT .* FROM `mysql_texas_game_info` WHERE game_id = \\? ORDER BY `mysql_texas_game_info`.`game_id` LIMIT \\?").
		WithArgs("missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"game_id"}))

	item, err := repo.GetGame(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetGame() error = %v", err)
	}
	if item != nil {
		t.Fatalf("GetGame() = %#v, want nil", item)
	}
}
