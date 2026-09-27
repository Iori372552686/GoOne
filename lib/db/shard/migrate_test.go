package shard

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newShardMigrateDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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
	t.Cleanup(func() { _ = pool.Close() })
	return db, mock
}

func TestMigrateUnshardedNoopWhenUnsharded(t *testing.T) {
	db, _ := newShardMigrateDB(t)
	rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: 1}
	n, err := MigrateUnsharded(context.Background(), db, rule, &[]g1_protocol.MysqlRoleData{}, func(int) uint64 { return 0 })
	if err != nil || n != 0 {
		t.Fatalf("unsharded rule should be noop, got n=%d err=%v", n, err)
	}
}

func TestMigrateUnshardedSkipsWhenBaseMissing(t *testing.T) {
	db, mock := newShardMigrateDB(t)
	mock.ExpectQuery("SELECT SCHEMA_NAME from Information_schema.SCHEMATA").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"SCHEMA_NAME"}).AddRow("g1_game"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM information_schema.tables WHERE table_schema = \\? AND table_name = \\? AND table_type = \\?").
		WithArgs("g1_game", "role_data", "BASE TABLE").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))

	rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: 16}
	n, err := MigrateUnsharded(context.Background(), db, rule, &[]g1_protocol.MysqlRoleData{}, func(int) uint64 { return 0 })
	if err != nil || n != 0 {
		t.Fatalf("missing base table should be noop, got n=%d err=%v", n, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateUnshardedRoutesAndRenames(t *testing.T) {
	db, mock := newShardMigrateDB(t)
	mock.ExpectQuery("SELECT SCHEMA_NAME from Information_schema.SCHEMATA").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"SCHEMA_NAME"}).AddRow("g1_game"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM information_schema.tables WHERE table_schema = \\? AND table_name = \\? AND table_type = \\?").
		WithArgs("g1_game", "role_data", "BASE TABLE").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))

	scanRows := sqlmock.NewRows([]string{"uid", "data", "update_time"}).
		AddRow(uint64(100), []byte("a"), int64(1)).
		AddRow(uint64(1), []byte("b"), int64(2))
	mock.ExpectQuery("SELECT \\* FROM `role_data`").WillReturnRows(scanRows)

	// 契约值：uid 100→role_data_1、uid 1→role_data_4（见 TestResolve_ContractValues）。
	// 注：uid 必须非零——gorm 对零值主键按自增假设跳列；生产 uid 永不为 0。
	// gorm 默认 SkipDefaultTransaction=false：每个 Create 自带 Begin/Commit。
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `role_data_1`").
		WithArgs([]byte("a"), int64(1), uint64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `role_data_4`").
		WithArgs([]byte("b"), int64(2), uint64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE `role_data` RENAME TO `role_data__legacy`")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	var loaded []g1_protocol.MysqlRoleData
	keyOf := func(i int) uint64 { return loaded[i].Uid }
	rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: 16}
	n, err := MigrateUnsharded(context.Background(), db, rule, &loaded, keyOf)
	if err != nil {
		t.Fatalf("MigrateUnsharded() error = %v", err)
	}
	if n != 2 {
		t.Fatalf("migrated = %d, want 2", n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateUnshardedRejectsBadRowsArg(t *testing.T) {
	db, mock := newShardMigrateDB(t)
	mock.ExpectQuery("SELECT SCHEMA_NAME from Information_schema.SCHEMATA").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"SCHEMA_NAME"}).AddRow("g1_game"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM information_schema.tables WHERE table_schema = \\? AND table_name = \\? AND table_type = \\?").
		WithArgs("g1_game", "role_data", "BASE TABLE").
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `role_data`").WillReturnRows(sqlmock.NewRows([]string{"uid"}))

	rule := &Rule{Name: "role_data", TableBase: "role_data", TableShards: 16}
	if _, err := MigrateUnsharded(context.Background(), db, rule, &g1_protocol.MysqlRoleData{}, func(int) uint64 { return 0 }); err == nil {
		t.Fatal("non-slice rows should be rejected")
	}
}

func TestLegacyName(t *testing.T) {
	if got := LegacyName("role_data"); got != "role_data__legacy" {
		t.Fatalf("LegacyName = %q", got)
	}
}
