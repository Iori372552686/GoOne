// by  Iori  2021/12/7
package orm

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/logger"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
)

/*
*  OrmSql
*  @Description: xorm struct
 */
type OrmSql struct {
	Engine  *xorm.EngineGroup
	Session *xorm.Session

	//private
	name      string
	driveName string
	dsn       []string
	syncFlag  bool
}

/**
* @Description: NewOrmSql
* @return: *OrmSql
* @Author: Iori
* @Date: 2022-05-21 16:50:04
**/
func NewOrmSql() *OrmSql {
	r := &OrmSql{}
	r.Engine = nil
	r.Session = nil

	return r
}

/**
* @Description:  添加链接实例
* @param: config
* @param: tables
* @return: *xorm.EngineGroup
* @return: error
* @Author: Iori
* @Date: 2022-05-21 16:49:36
**/
func (self *OrmSql) AddInstance(conf Config, tables ...interface{}) (*xorm.EngineGroup, error) {
	self.name = conf.IndexName
	self.syncFlag = conf.InitFlag
	self.driveName = conf.DriveName

	self.dsn = append(self.dsn,
		fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=3s&parseTime=true&loc=Local&charset=utf8",
			conf.Master.User,
			conf.Master.Password,
			conf.Master.IP,
			conf.Master.Port,
			conf.Master.DBName),
	)
	for _, slaveCfg := range conf.Slaves {
		self.dsn = append(self.dsn,
			fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=3s&parseTime=true&loc=Local&charset=utf8",
				slaveCfg.User,
				slaveCfg.Password,
				slaveCfg.IP,
				slaveCfg.Port,
				slaveCfg.DBName),
		)
	}

	logger.Infof("init data source | %v", redactDSNs(self.dsn))
	impl, err := xorm.NewEngineGroup(self.driveName, self.dsn)
	if err != nil {
		logger.Errorf("data source init error | %v", err.Error())
		return nil, err
	}

	//opt
	impl.ShowSQL(conf.ShowSQL)
	impl.SetMaxIdleConns(conf.MaxIdle)
	impl.SetMaxOpenConns(conf.MaxOpen)
	impl.ShowExecTime(true)
	self.Engine = impl
	registerOrmMetrics(self.name, self)

	err = self.SyncTables(tables...)
	if err != nil {
		// SyncTables 失败时关闭已创建的 Engine，避免连接泄漏。
		_ = impl.Close()
		return nil, err
	}

	//check
	finishPing := beginXormPingObserve(self.name)
	err = impl.Ping()
	finishPing(err)
	if err != nil {
		defer impl.Close()
		logger.Errorf("data source Ping() error | %v", err.Error())
		return nil, err
	}

	self.Session = self.Engine.NewSession()
	return impl, nil
}

/**
* @Description: 恢复conn
* @return: err
* @Author: Iori
* @Date: 2022-05-21 16:48:59
**/
func (self *OrmSql) refresh() (err error) {
	self.Session.Close()
	self.Engine.Close()
	self.Engine = nil
	self.Session = nil

	self.Engine, err = xorm.NewEngineGroup(self.driveName, self.dsn)
	if err != nil {
		return err
	}

	self.Session = self.Engine.NewSession()
	return nil
}

/**
 * @Description: 连接监控器
 * @Author: Iori
 * @Date: 2022-05-21 16:48:40
 **/
func (self *OrmSql) MonitorConn() {

	finishPing := beginXormPingObserve(self.name)
	err := self.Engine.Ping()
	finishPing(err)
	if err != nil {
		err = self.refresh()
		if err != nil {
			logger.Errorf("OrmSql - MonitorConn  refresh() error | %v", err.Error())
		}
	}
}

/**
 * @Description: 同步创建表與字段
 * @Author: Iori
 * @Date: 2022-05-21 16:53:20
 **/
func (self *OrmSql) SyncTables(tables ...interface{}) error {
	if self.syncFlag && tables != nil {

		for _, table := range tables {
			//sync  table
			err := self.Engine.Sync(
				table,
			)
			if err != nil {
				logger.Errorf("data source init error | %v", err.Error())
				return err
			}
		}
		logger.Infof("## init [%s] db table ## | %#v", self.name, tables)

	}

	return nil
}

// Close 关闭 Engine 与 Session，释放数据库连接。幂等：Engine 为 nil 时安全返回
//（OrmSql 增加显式 Close，使资源由 Component 统一关闭）。
func (self *OrmSql) Close() error {
	if self.Session != nil {
		self.Session.Close()
		self.Session = nil
	}
	if self.Engine != nil {
		_ = self.Engine.Close()
		self.Engine = nil
	}
	return nil
}
