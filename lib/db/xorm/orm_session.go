package orm

import (
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/go-xorm/xorm"
)

type ORMOperation func(session *xorm.Session) error

/**
* @Description:  xorm 事务处理
* @param: func  业务函数
* @return: err
* @Author: Iori
* @Date: 2022-11-29 18:33:00
**/
func (self *OrmSql) Transaction(f ORMOperation) (err error) {
	//session := self.Session.Begin()
	session := self.Engine.NewSession()

	err = session.Begin()
	if err != nil {
		return
	}

	defer func() {
		if p := recover(); p != nil {
			logger.Errorf("Transaction recover rollback:%s", p)
			session.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			logger.Errorf("Transaction error rollback:%s", err.Error())
			session.Rollback() // err is non-nil; don't change it
		} else {
			err = session.Commit() // err is nil; if Commit returns error update err
		}
	}()

	err = f(session) //用于defer闭包检查。
	return err
}
