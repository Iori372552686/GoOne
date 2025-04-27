package test

import (
	"testing"

	"github.com/Iori372552686/GoOne/common/gconf"
	orm "github.com/Iori372552686/GoOne/lib/db/xorm"
	"github.com/Iori372552686/GoOne/lib/util/marshal"
	"github.com/Iori372552686/GoOne/src/mysqlsvr/manager"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

func TestMain(m *testing.M) {
	if err := marshal.LoadConfFile("./server_config.yaml", &gconf.MySqlSvrCfg); err != nil {
		panic(err)
	}

	if err := orm.Orm_Mgr.InitAndRun(gconf.MySqlSvrCfg.OrmConf, manager.GetTables()...); err != nil {
		panic(err)
	}
	m.Run()
}

func TestDb(t *testing.T) {
	req := &g1_protocol.QueryRoomInfoReq{
		RoomId:    101553,
		TableId:   174993957439143947,
		GameType:  1,
		RoomStage: 3,
	}
	session := orm.Orm_Mgr.GetOrmEngine().Where("room_id = ?", req.RoomId)
	var rooms []g1_protocol.MysqlTexasRoomInfo
	err := session.OrderBy("id").Find(&rooms)
	t.Log("----------->", err)
}

func TestFind(t *testing.T) {
	cli := orm.Orm_Mgr.GetOrmEngine().NewSession()
	defer cli.Close()

	olds := []*g1_protocol.MysqlTexasRoomInfo{}
	err := cli.Where("game_type = ? and room_stage = ?", 1, 1).Find(&olds)
	t.Log(err, "=--------", olds[0], olds[1])
}
