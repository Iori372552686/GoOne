// by  Iori  2022/1/4
package ssdb

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/seefan/gossdb/v2"
	"github.com/seefan/gossdb/v2/conf"
	"github.com/seefan/gossdb/v2/pool"
)

type Ssdb struct {
	Engine *pool.Connectors
}

func NewSsdbEngine() *Ssdb {
	r := &Ssdb{}
	return r
}

func (self *Ssdb) AddInstance(sdc Config) (*pool.Connectors, error) {
	se, err := gossdb.NewPool(&conf.Config{
		Host:         sdc.IP,
		Port:         sdc.Port,
		MaxWaitSize:  sdc.MaxWaitSize,
		PoolSize:     5,
		MinPoolSize:  5,
		MaxPoolSize:  sdc.MaxPool,
		AutoClose:    sdc.AutoClose,
		Password:     sdc.Password,
		HealthSecond: sdc.HealthSecond,
	})

	if err != nil {
		return nil, err
	}
	return se, nil
}
