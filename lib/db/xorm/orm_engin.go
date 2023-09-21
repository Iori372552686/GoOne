// by  Iori  2021/12/7
package orm

import (
	"GoOne/lib/api/logger"
	"fmt"

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
* @param: conf
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
		fmt.Sprintf("%s:%s@tcp(%s)/%s?timeout=3s&parseTime=true&loc=Local&charset=utf8",
			conf.Master.User,
			conf.Master.Password,
			conf.Master.IP,
			conf.Master.DBName),
	)
	for _, slaveCfg := range conf.Slaves {
		self.dsn = append(self.dsn,
			fmt.Sprintf("%s:%s@tcp(%s)/%s?timeout=3s&parseTime=true&loc=Local&charset=utf8",
				slaveCfg.User,
				slaveCfg.Password,
				slaveCfg.IP,
				slaveCfg.DBName),
		)
	}

	logger.Infof("init data source | %v", self.dsn)
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

	err = self.SyncTables(tables...)
	if err != nil {
		return nil, err
	}

	//check
	err = impl.Ping()
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
	err := self.Engine.Ping()
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
