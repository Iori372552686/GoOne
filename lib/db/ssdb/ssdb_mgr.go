package ssdb

import (
	"github.com/Iori372552686/GoOne/common/gfunc"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/seefan/gossdb/v2/pool"
)

type SsdbMgr struct {
	Instances map[string]*Ssdb

	//private
	lastTick int64
}

func NewSsdbMgr() *SsdbMgr {
	r := &SsdbMgr{}
	r.Instances = make(map[string]*Ssdb)

	return r
}

func (self *SsdbMgr) SetSsdb(key string, o *Ssdb) {
	self.Instances[key] = o
}

func (self *SsdbMgr) GetSsdb(keys ...string) *Ssdb {
	if len(keys) == 0 {
		return self.Instances["default"]
	} else {
		return self.Instances[keys[0]]
	}
}

func (self *SsdbMgr) GetEngine(keys ...string) *pool.Connectors {
	if len(keys) == 0 {
		return self.Instances["default"].Engine
	} else {
		return self.Instances[keys[0]].Engine
	}
}

func (self *SsdbMgr) InitAndRun(cfgs []Config) error {
	logger.Infof("SsdbMgr   InsInit.. ")

	for _, ds := range cfgs {
		ssdb := NewSsdbEngine()
		_, err := ssdb.AddInstance(ds)
		if err != nil {
			return err
		}

		self.SetSsdb(ds.Key, ssdb)
	}

	logger.Infof("SsdbMgr   InsInit... Done !")
	return nil
}

// tick
func (self *SsdbMgr) Tick(nowMs int64) {
	defer gfunc.CheckRecover()
	return
}
