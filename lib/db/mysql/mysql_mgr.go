package mysql

import (
	"GoOne/lib/api/logger"
	"database/sql"
	"fmt"

	"sync"

	_ "github.com/go-sql-driver/mysql"
)

type MysqlMgr struct {
	dbs sync.Map // map[uint32]*sql.DB
}

func NewMysqlMgr() *MysqlMgr {
	m := new(MysqlMgr)
	return m
}

func (m *MysqlMgr) Destroy() {
	m.dbs.Range(func(key, value interface{}) bool {
		db, ok := value.(*sql.DB)
		if ok && db != nil {
			_ = db.Close()
		}
		return true
	})

	m.dbs = sync.Map{}
}

func (m *MysqlMgr) AddInstance(id uint32, ip string, port int16, user, pass, schema string) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, pass, ip, port, schema)
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open a mysql instance {dsn:%s} | %v", dsn, err)
	}

	if v, exist := m.dbs.Load(id); exist {
		oldConn, ok := v.(*sql.DB)
		if ok {
			logger.Warningf("overwrite a mysql instance")
			_ = oldConn.Close()
			m.dbs.Delete(id)
		}
	}

	m.dbs.Store(id, conn)

	return nil
}

func (m *MysqlMgr) GetDb(id uint32) *sql.DB {
	if v, exist := m.dbs.Load(id); exist {
		db, ok := v.(*sql.DB)
		if ok && db != nil {
			return db
		}
	}

	logger.Errorf("failed to get a mysql db")

	return nil
}

func (m *MysqlMgr) Execute(id uint32, query string, args ...interface{}) (sql.Result, error) {
	db := m.GetDb(id)
	if db == nil {
		return nil, fmt.Errorf("execute on an non-exist db {id:%v, q:%v}", id, query)
	}

	return db.Exec(query, args...)
}

func (m *MysqlMgr) Query(id uint32, query string, args ...interface{}) (*sql.Rows, error) {
	db := m.GetDb(id)
	if db == nil {
		return nil, fmt.Errorf("execute on an non-exist db {id:%v, q:%v}", id, query)
	}

	return db.Query(query, args...)
}
