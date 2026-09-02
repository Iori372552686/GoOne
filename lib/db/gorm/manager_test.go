package gormdb

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func TestGetDBReturnsMissingInstanceError(t *testing.T) {
	_, err := NewManager().GetDB("missing")
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("GetDB() error = %v, want ErrInstanceNotFound", err)
	}
}

func TestDBResolverRoutesExplicitReadAndWrite(t *testing.T) {
	masterDB, masterMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	replicaDB, replicaMock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	masterMock.ExpectPing()
	replicaMock.ExpectPing()

	pools := []*sql.DB{masterDB, replicaDB}
	m := NewManager()
	m.openPool = func(string) (*sql.DB, error) {
		pool := pools[0]
		pools = pools[1:]
		return pool, nil
	}
	config := Config{
		IndexName: "default", DriveName: "mysql",
		Master:  &DbInfo{IP: "master", Port: 3306, User: "game", DBName: "goone"},
		Slaves:  []*DbInfo{{IP: "replica", Port: 3306, User: "game", DBName: "goone"}},
		MaxIdle: 1, MaxOpen: 2,
	}
	if err := m.InitAndRun(context.Background(), []Config{config}); err != nil {
		t.Fatalf("InitAndRun() error = %v", err)
	}

	db, err := m.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	masterMock.ExpectExec("UPDATE role_info").WillReturnResult(sqlmock.NewResult(0, 1))
	if err := db.Clauses(dbresolver.Write).Exec("UPDATE role_info SET name=? WHERE uid=?", "name", 1).Error; err != nil {
		t.Fatalf("write error = %v", err)
	}
	replicaMock.ExpectQuery("SELECT 7").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(7))
	var value int
	if err := db.Clauses(dbresolver.Read).Raw("SELECT 7").Scan(&value).Error; err != nil {
		t.Fatalf("read error = %v", err)
	}
	if value != 7 {
		t.Fatalf("read value = %d, want 7", value)
	}

	masterMock.ExpectClose()
	replicaMock.ExpectClose()
	if err := m.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := masterMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("master expectations: %v", err)
	}
	if err := replicaMock.ExpectationsWereMet(); err != nil {
		t.Fatalf("replica expectations: %v", err)
	}
}

func TestTransactionCommitsAndRollsBack(t *testing.T) {
	pool, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectPing()
	m := NewManager()
	m.openPool = func(string) (*sql.DB, error) { return pool, nil }
	config := Config{
		IndexName: "default", DriveName: "mysql",
		Master:  &DbInfo{IP: "master", Port: 3306, User: "game", DBName: "goone"},
		MaxIdle: 1, MaxOpen: 2,
	}
	if err := m.InitAndRun(context.Background(), []Config{config}); err != nil {
		t.Fatalf("InitAndRun() error = %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO audit").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := m.Transaction(context.Background(), "default", func(tx *gorm.DB) error {
		return tx.Exec("INSERT INTO audit(value) VALUES (?)", 1).Error
	}); err != nil {
		t.Fatalf("commit transaction error = %v", err)
	}

	rollbackErr := errors.New("reject write")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO audit").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectRollback()
	err = m.Transaction(context.Background(), "default", func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT INTO audit(value) VALUES (?)", 2).Error; err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("rollback transaction error = %v, want %v", err, rollbackErr)
	}

	mock.ExpectClose()
	_ = m.Close()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("transaction expectations: %v", err)
	}
}

func TestGetDBUsesDefaultName(t *testing.T) {
	m := NewManager()
	_, err := m.GetDB()
	if !errors.Is(err, ErrInstanceNotFound) {
		t.Fatalf("GetDB() error = %v, want ErrInstanceNotFound", err)
	}
}
