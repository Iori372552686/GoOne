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

func TestRoleInfoSchemaCompatibility(t *testing.T) {
	parsed := parseSchema(t, &g1_protocol.MysqlRoleInfo{})
	if parsed.Table != "mysql_role_info" {
		t.Fatalf("table = %q, want mysql_role_info", parsed.Table)
	}
	assertField(t, parsed, "Uid", "uid", "bigint", true, false)
	assertField(t, parsed, "Name", "name", "varchar(25)", false, false)

	indexes := parsed.ParseIndexes()
	found := false
	for _, index := range indexes {
		if index.Name == "IDX_role_info_name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("name index missing: %#v", indexes)
	}
}

func TestRoleDataSchemaCompatibility(t *testing.T) {
	parsed := parseSchema(t, &g1_protocol.MysqlRoleData{})
	if parsed.Table != "mysql_role_data" {
		t.Fatalf("table = %q, want mysql_role_data", parsed.Table)
	}
	assertField(t, parsed, "Uid", "uid", "bigint", true, false)
	assertField(t, parsed, "Data", "data", "longblob", false, false)
	assertField(t, parsed, "UpdateTime", "update_time", "bigint", false, false)
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
