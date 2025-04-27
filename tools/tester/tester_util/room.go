package tester_util

import (
	"fmt"

	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

type Room struct {
	Role *Role
}

func (r *Room) QuickStart() ([]byte, error) {
	body := &g1_protocol.QuickStartReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		CoinType:  g1_protocol.CoinType_COIN_NONE,
		Stage:     g1_protocol.RoomStage_Free,
		ConnBusId: uint32(231251),
	}

	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		fmt.Printf("quickStart Room error: %v", err)
		return nil, err
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      r.Role.GetUid(),
		Cmd:      uint32(g1_protocol.CMD_MAIN_GAME_QUICK_START_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}

	return append(header.ToBytes(), bodyBytes...), nil
}

func (r *Room) GetListRoom() ([]byte, error) {
	body := &g1_protocol.RoomListReq{
		GameId:    g1_protocol.GameTypeId_TEXAS_NORMAL,
		Stage:     g1_protocol.RoomStage_Free,
		RoomIds:   make([]uint32, 0),
		PageIndex: 0,
		PageSize:  10,
		SortType:  g1_protocol.RoomSortType_SORT_TYPE_ID,
		CoinType:  g1_protocol.CoinType_COIN_NONE,
	}

	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		fmt.Printf("query Room error: %v", err)
		return nil, err
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      r.Role.GetUid(),
		Cmd:      uint32(g1_protocol.CMD_MAIN_GAME_ROOM_LIST_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}

	return append(header.ToBytes(), bodyBytes...), nil
}

func (r *Room) GetJoinRoom() ([]byte, error) {
	body := &g1_protocol.JoinRoomReq{
		RoomId:    uint64(12233),
		ConnBusId: uint32(231251),
	}
	bodyBytes, err := proto.Marshal(body)

	if err != nil {
		fmt.Printf("join Room error: %v", err)
		return nil, err
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      r.Role.GetUid(),
		Cmd:      uint32(g1_protocol.CMD_MAIN_GAME_JOIN_ROOM_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}
	return append(header.ToBytes(), bodyBytes...), nil
}

func (r *Room) GetCreateRoom() ([]byte, error) {
	body := &g1_protocol.CreateRoomReq{
		GameId: g1_protocol.GameTypeId_TEXAS_NORMAL,
		Name:   "测试rummy房间",
	}

	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		fmt.Printf("createRoom error: %v", err)
		return nil, err
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      r.Role.GetUid(),
		Cmd:      uint32(g1_protocol.CMD_MAIN_GAME_CREATE_ROOM_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}
	return append(header.ToBytes(), bodyBytes...), nil
}
