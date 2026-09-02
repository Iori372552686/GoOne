package gormdb

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Iori372552686/GoOne/lib/internal/itest"
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestAutoMigratePreservesExistingSchemaAndDataIntegration(t *testing.T) {
	if !itest.Enabled() {
		t.Skip("set GOONE_INTEGRATION=1 to run integration tests")
	}
	dsn := os.Getenv("GOONE_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("GOONE_MYSQL_DSN is required when GOONE_INTEGRATION=1")
	}
	parsed, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse GOONE_MYSQL_DSN: %v", err)
	}
	if parsed.Net != "tcp" {
		t.Fatalf("GOONE_MYSQL_DSN network = %q, want tcp", parsed.Net)
	}
	itest.Require(t, parsed.Addr)
	host, portText, err := net.SplitHostPort(parsed.Addr)
	if err != nil {
		t.Fatalf("split MySQL address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	serverConfig := parsed.Clone()
	serverConfig.DBName = ""
	server, err := sql.Open("mysql", serverConfig.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	databaseName := fmt.Sprintf("goone_itest_%d", time.Now().UnixNano())
	if _, err := server.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("create isolated integration database: %v", err)
	}
	t.Cleanup(func() { _, _ = server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`") })

	config := Config{
		IndexName: "default", DriveName: "mysql", InitFlag: true, MaxIdle: 1, MaxOpen: 4,
		Master: &DbInfo{IP: host, Port: port, User: parsed.User, Password: parsed.Passwd, DBName: databaseName},
	}
	models := []interface{}{new(g1_protocol.MysqlTexasRoomInfo), new(g1_protocol.MysqlTexasPlayerInfo), new(g1_protocol.MysqlTexasGameInfo)}
	m := NewManager()
	if err := m.InitAndRun(context.Background(), []Config{config}, models...); err != nil {
		t.Fatalf("AutoMigrate empty database: %v", err)
	}
	db, err := m.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range models {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("AutoMigrate did not create table for %T", model)
		}
	}
	if err := db.Exec("ALTER TABLE mysql_texas_room_info ADD COLUMN legacy_extra varchar(32) NULL").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO mysql_texas_room_info (room_id, game_type, room_stage, blind, create_time, legacy_extra) VALUES (?, ?, ?, ?, ?, ?)", 10, 1, 1, "1/2", 100, "keep-me").Error; err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}

	m = NewManager()
	if err := m.InitAndRun(context.Background(), []Config{config}, models...); err != nil {
		t.Fatalf("AutoMigrate existing schema: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	db, err = m.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn("mysql_texas_room_info", "legacy_extra") {
		t.Fatal("AutoMigrate removed legacy_extra column")
	}
	var legacyValue string
	if err := db.Raw("SELECT legacy_extra FROM mysql_texas_room_info WHERE room_id = ?", 10).Scan(&legacyValue).Error; err != nil {
		t.Fatal(err)
	}
	if legacyValue != "keep-me" {
		t.Fatalf("existing data changed: legacy_extra = %q", legacyValue)
	}
}
