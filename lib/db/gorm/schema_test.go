package gormdb

import (
	"sync"
	"testing"

	g1_protocol "github.com/Iori372552686/g1_common/protocol"
	"gorm.io/gorm/schema"
)

func parseSchema(t *testing.T, model interface{}) *schema.Schema {
	t.Helper()
	parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{SingularTable: true})
	if err != nil {
		t.Fatalf("schema.Parse() error = %v", err)
	}
	return parsed
}

func TestTexasRoomSchemaCompatibility(t *testing.T) {
	parsed := parseSchema(t, &g1_protocol.MysqlTexasRoomInfo{})
	if parsed.Table != "mysql_texas_room_info" {
		t.Fatalf("table = %q, want mysql_texas_room_info", parsed.Table)
	}
	assertField(t, parsed, "Id", "id", "bigint", true, false)
	assertField(t, parsed, "RoomId", "room_id", "bigint", false, true)

	indexes := parsed.ParseIndexes()
	found := false
	for _, index := range indexes {
		if index.Name == "IDX_mysql_texas_room_info_room_id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("room_id index missing: %#v", indexes)
	}
}

func TestTexasGameSchemaCompatibility(t *testing.T) {
	parsed := parseSchema(t, &g1_protocol.MysqlTexasGameInfo{})
	assertField(t, parsed, "GameId", "game_id", "varchar(125)", true, false)
	assertField(t, parsed, "GameDetail", "game_detail", "blob", false, false)
}

func assertField(t *testing.T, parsed *schema.Schema, name, column, columnType string, primaryKey, notNull bool) {
	t.Helper()
	field := parsed.LookUpField(name)
	if field == nil {
		t.Fatalf("field %s not found", name)
	}
	if field.DBName != column {
		t.Fatalf("field %s DBName = %q, want %q", name, field.DBName, column)
	}
	if field.TagSettings["TYPE"] != columnType {
		t.Fatalf("field %s type = %q, want %q", name, field.TagSettings["TYPE"], columnType)
	}
	if field.PrimaryKey != primaryKey {
		t.Fatalf("field %s PrimaryKey = %v, want %v", name, field.PrimaryKey, primaryKey)
	}
	if field.NotNull != notNull {
		t.Fatalf("field %s NotNull = %v, want %v", name, field.NotNull, notNull)
	}
}
