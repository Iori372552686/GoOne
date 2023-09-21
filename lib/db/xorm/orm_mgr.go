// by  Iori  2021/12/7
package orm

import (
	"GoOne/common"
	"GoOne/lib/api/datetime"
	"GoOne/lib/api/logger"
	"github.com/go-xorm/xorm"

	_ "github.com/go-sql-driver/mysql"
)

var Orm_Mgr = NewOrmMgr()

type OrmMgr struct {
	XormEngine map[string]*OrmSql

	//private
	lastTick int64
}

func NewOrmMgr() *OrmMgr {
	r := &OrmMgr{}
	r.XormEngine = make(map[string]*OrmSql)

	return r
}

func (self *OrmMgr) SetOrm(key string, o *OrmSql) {
	self.XormEngine[key] = o
}

func (self *OrmMgr) GetOrm(keys ...string) *OrmSql {
	if len(keys) == 0 {
		return self.XormEngine["default"]
	} else {
		return self.XormEngine[keys[0]]
	}
}

func (self *OrmMgr) GetOrmEngine(dbName ...string) *xorm.EngineGroup {
	orm := &OrmSql{}

	if len(dbName) == 0 {
		orm = self.XormEngine["default"]
	} else {
		orm = self.XormEngine[dbName[0]]
	}

	if orm == nil {
		return nil
	}

	return orm.Engine
}

/**
* @Description:  init
* @param: dbIns
* @param: tables
* @return: error
* @Author: Iori
**/
func (self *OrmMgr) InitAndRun(dbIns []Config, tables ...interface{}) error {
	logger.Infof("OrmMgr   InsInit.. | %#v", tables)

	for _, ds := range dbIns {
		orm := NewOrmSql()
		_, err := orm.AddInstance(ds, tables...)
		if err != nil {
			return err
		}

		self.SetOrm(ds.IndexName, orm)
	}

	logger.Infof("OrmMgr   InsInit... Done !")
	return nil
}

/**
* @Description: tick
* @param: nowMs
* @Author: Iori
* @Date: 2022-10-13 11:29:17
**/
func (self *OrmMgr) Tick(nowMs int64) {
	defer common.CheckRecover()

	if (nowMs - self.lastTick) > 30*datetime.MS_PER_SECOND {
		//logger.Infof("OrmMgr   Tick.. ")

		for _, engine := range self.XormEngine {
			engine.MonitorConn()
		}

		self.lastTick = nowMs
	}

	return
}
