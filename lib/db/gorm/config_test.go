package gormdb

import (
	"strings"
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestConfigValidateAcceptsExistingMySQLShape(t *testing.T) {
	config := Config{
		IndexName: "default",
		DriveName: "mysql",
		Master: &DbInfo{
			IP: "127.0.0.1", Port: 3306, User: "game", Password: "secret", DBName: "goone",
		},
		MaxIdle: 5,
		MaxOpen: 20,
	}
	if err := config.validate(); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestConfigValidateRejectsNonMySQLDriver(t *testing.T) {
	config := Config{
		IndexName: "default",
		DriveName: "postgres",
		Master:    &DbInfo{IP: "127.0.0.1", Port: 5432, User: "game", DBName: "goone"},
	}
	if err := config.validate(); err == nil {
		t.Fatal("validate() error = nil, want unsupported driver error")
	}
}

func TestBuildDSNEscapesCredentialsAndKeepsCompatibilityOptions(t *testing.T) {
	dsn := buildDSN(&DbInfo{
		IP: "db.internal", Port: 3306, User: "game", Password: "p@ss/word", DBName: "goone",
	})
	parsed, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("ParseDSN(%q) error = %v", redactDSN(dsn), err)
	}
	if parsed.User != "game" || parsed.Passwd != "p@ss/word" {
		t.Fatalf("credentials did not round trip: user=%q password=%q", parsed.User, parsed.Passwd)
	}
	if !strings.Contains(dsn, "charset=utf8mb4") || !parsed.ParseTime || parsed.Loc == nil {
		t.Fatalf("compatibility options missing: %#v", parsed)
	}
	if strings.Contains(redactDSN(dsn), "p@ss/word") {
		t.Fatal("redactDSN exposed password")
	}
}
