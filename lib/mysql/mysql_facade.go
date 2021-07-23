package mysql

import (
	"database/sql"
	"fmt"

	`GoOne/lib/logger`
)

type IFacde interface {
	Init(ip string, port int16, user, password, schema string) error
	Execute(statement string) error
	Select(q string)
}


type Facade struct {
	db *sql.DB
}

func (f *Facade) Init(ip string, port int16, user, password, schema string) error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, password, ip, port, schema)
	f.db, err = sql.Open("mysql", dsn)
	if err != nil {
		logger.Errorf("Failed to open a mysql {dsn:%s} | %v", dsn, err)
		return err
	}

	return nil
}

//func (f *Facade) Execute(statement string) error {
//	if f.db == nil {
//		return misc.LogError("Execute a sql on an empty db")
//	}
//
//	insert, err := f.db.Query(statement)
//}
