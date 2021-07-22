package info

import (
	"fmt"

	`bian/src/bian_newFrame/lib/algorithm`
	`bian/src/bian_newFrame/lib/redis`
	g1_protocol `bian/src/bian_newFrame/protobuf/protocol`
	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

const (
	CACHE_SIZE = 10000
)


type InfoMgr struct {
	data *algorithm.LRUCache

	RedisMgr *redis.RedisMgr
}

func NewInfoMgr() *InfoMgr {
	mgr := new(InfoMgr)
	mgr.data = algorithm.NewLRUCache(CACHE_SIZE)
	mgr.RedisMgr = redis.NewRedisMgr()
	return mgr
}

func (m *InfoMgr) GetInfo(uidList *[]uint64) (*[]*g1_protocol.PbRoleBriefInfo, int) {
	missUid := make([]uint64, 0)
	briefs := make([]*g1_protocol.PbRoleBriefInfo, 0)
	for _, uid := range *uidList {
		v, exist, e := m.data.Get(uid)
		if e != nil {
			return nil, int(g1_protocol.ErrorCode_ERR_FAIL)
		}
		if !exist {
			missUid = append(missUid, uid)
		} else {
			briefs = append(briefs, v.(*g1_protocol.PbRoleBriefInfo))
		}
	}

	// 如果miss了一部分，则去db拉取
	if len(missUid) > 0 {
		rsp, _ := m.loadBriefFromDB(missUid)
		if rsp != nil {
			for _, brief := range *rsp {
				briefs = append(briefs, brief)
				_ = m.data.Set(brief.Uid, brief)
			}
		}
	}

	return &briefs, 0
}

func (m *InfoMgr) SetInfo(uid uint64, brief *g1_protocol.PbRoleBriefInfo) int {
	_ = m.data.Set(uid, brief)
	return m.saveBriefToDB(uid, brief)
}


func (m *InfoMgr) loadBriefFromDB(uidList []uint64) (*[]*g1_protocol.PbRoleBriefInfo, int) {
	dbType := uint32(g1_protocol.DBType_DB_TYPE_BRIEF_INFO)
	keys := make([]string, 0, len(uidList))
	for _, v := range uidList {
		key := fmt.Sprintf("%d:%d", dbType, v)
		keys = append(keys, key)
	}
	rsp, err := m.RedisMgr.MGetBytes(dbType, keys)
	if err != nil {
		glog.Error("get redis brief error: ", err)
		return nil, int(g1_protocol.ErrorCode_ERR_DB)
	}
	ret := make([]*g1_protocol.PbRoleBriefInfo, 0, len(uidList))
	for _, v := range rsp {
		brief := &g1_protocol.PbRoleBriefInfo{}
		_ = proto.Unmarshal([]byte(v), brief)
		ret = append(ret, brief)
	}
	return &ret, 0
}

func (m *InfoMgr) saveBriefToDB(uid uint64, brief *g1_protocol.PbRoleBriefInfo) int {
	dbType := uint32(g1_protocol.DBType_DB_TYPE_BRIEF_INFO)
	key := fmt.Sprintf("%d:%d", dbType, uid)
	data, _ := proto.Marshal(brief)
	err := m.RedisMgr.SetBytes(dbType, key, data)
	if err != nil {
		glog.Errorf("set redis brief err: ", err)
		return int(g1_protocol.ErrorCode_ERR_DB)
	}
	return 0
}

