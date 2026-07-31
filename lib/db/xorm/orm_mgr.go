// by  Iori  2021/12/7
package orm

import (
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/module/gfunc"
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

	// 记录已成功添加的实例，用于中途失败时逆序关闭（多实例初始化失败
	// 必须回滚已成功 Engine，避免连接泄漏）。
	added := make([]string, 0, len(dbIns))
	for _, ds := range dbIns {
		orm := NewOrmSql()
		_, err := orm.AddInstance(ds, tables...)
		if err != nil {
			// 逆序关闭已成功添加的 Engine。
			for i := len(added) - 1; i >= 0; i-- {
				_ = self.XormEngine[added[i]].Close()
				delete(self.XormEngine, added[i])
			}
			return err
		}

		self.SetOrm(ds.IndexName, orm)
		added = append(added, ds.IndexName)
	}

	logger.Infof("OrmMgr   InsInit... Done !")
	return nil
}

// Close 关闭所有 ORM Engine，释放数据库连接。幂等（OrmManager 增加 Close）。
func (self *OrmMgr) Close() error {
	for name, orm := range self.XormEngine {
		if orm != nil {
			_ = orm.Close()
		}
		delete(self.XormEngine, name)
	}
	return nil
}

/**
* @Description: tick
* @param: nowMs
* @Author: Iori
* @Date: 2022-10-13 11:29:17
**/
func (self *OrmMgr) Tick(nowMs int64) {
	defer gfunc.CheckRecover()

	if (nowMs - self.lastTick) > 30*datetime.MS_PER_SECOND {
		//logger.Infof("OrmMgr   Tick.. ")

		for _, engine := range self.XormEngine {
			engine.MonitorConn()
		}

		self.lastTick = nowMs
	}

	return
}
