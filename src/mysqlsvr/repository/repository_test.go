package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Iori372552686/GoOne/lib/db/shard"
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

func TestUpsertRoleDataInsertsWhenMissing(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `role_data` WHERE uid = ? LIMIT ? FOR UPDATE")).
		WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `role_data` (`data`,`update_time`,`uid`) VALUES (?,?,?)")).
		WithArgs([]byte("snapshot"), int64(1000), uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpsertRoleData(context.Background(), 7, []byte("snapshot"), 1000); err != nil {
		t.Fatalf("UpsertRoleData() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertRoleDataRejectsStaleUpdate(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `role_data` WHERE uid = ? LIMIT ? FOR UPDATE")).
		WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid", "data", "update_time"}).AddRow(7, []byte("newer"), 2000))
	mock.ExpectRollback()

	err := repo.UpsertRoleData(context.Background(), 7, []byte("older"), 1000)
	if !errors.Is(err, ErrStaleUpdate) {
		t.Fatalf("UpsertRoleData() error = %v, want ErrStaleUpdate", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertRoleDataAllowsEqualTimestamp(t *testing.T) {
	// 相同 update_time 放行（同 ms 重写幂等）：只有严格更旧才拒绝。
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `role_data` WHERE uid = ? LIMIT ? FOR UPDATE")).
		WithArgs(uint64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid", "data", "update_time"}).AddRow(7, []byte("same"), 1000))
	mock.ExpectExec("UPDATE `role_data` SET `data`=\\?,`update_time`=\\? WHERE uid = \\?").
		WithArgs([]byte("same2"), int64(1000), uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpsertRoleData(context.Background(), 7, []byte("same2"), 1000); err != nil {
		t.Fatalf("UpsertRoleData() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertRoleDataValidatesArgs(t *testing.T) {
	repo, _, pool := newRepositoryTestDB(t)
	defer pool.Close()
	if err := repo.UpsertRoleData(context.Background(), 0, []byte("x"), 1); err == nil {
		t.Error("uid=0 should be rejected")
	}
	if err := repo.UpsertRoleData(context.Background(), 7, nil, 1); err == nil {
		t.Error("empty payload should be rejected")
	}
}

func TestLoadRoleDataMapsNotFoundToNil(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectQuery("SELECT .* FROM `role_data` WHERE uid = \\? LIMIT \\?").
		WithArgs(uint64(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid"}))

	item, err := repo.LoadRoleData(context.Background(), 9)
	if err != nil {
		t.Fatalf("LoadRoleData() error = %v", err)
	}
	if item != nil {
		t.Fatalf("LoadRoleData() = %#v, want nil", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRoleDataReturnsRow(t *testing.T) {
	repo, mock, pool := newRepositoryTestDB(t)
	defer pool.Close()
	mock.ExpectQuery("SELECT .* FROM `role_data` WHERE uid = \\? LIMIT \\?").
		WithArgs(uint64(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"uid", "data", "update_time"}).AddRow(9, []byte("blob"), int64(42)))

	item, err := repo.LoadRoleData(context.Background(), 9)
	if err != nil {
		t.Fatalf("LoadRoleData() error = %v", err)
	}
	if item == nil || string(item.Data) != "blob" || item.UpdateTime != 42 {
		t.Fatalf("LoadRoleData() = %#v, want uid=9 data=blob ts=42", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRoleDataShardedTableRouting(t *testing.T) {
	// TableShards>1 时表名必须带后缀且同一 uid 稳定（公式契约见 lib/db/shard）。
	repo := NewWithShard(testDBProvider{}, &shard.Rule{Name: "role_data", TableBase: "role_data", TableShards: 16})
	table, _, err := repo.roleDataTable(9527)
	if err != nil {
		t.Fatal(err)
	}
	if table != "role_data_1" {
		t.Fatalf("roleDataTable(9527) = %q, want role_data_1", table)
	}
}
