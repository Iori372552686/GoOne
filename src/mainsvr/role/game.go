package role

import (
	util "github.com/Iori372552686/GoOne/lib/util/slices"
	pb "github.com/Iori372552686/g1_common/protocol"
)

func (r *Role) AddPlayRoomID(roomId uint64) pb.ErrorCode {
	info := r.PbRole.GameInfo

	if info.PlayRoomIds == nil {
		info.PlayRoomIds = make([]uint64, 0)
	}

	for _, v := range info.PlayRoomIds {
		if v == roomId {
			return pb.ErrorCode_ERR_OK
		}
	}

	info.PlayRoomIds = util.InsertAtTail(info.PlayRoomIds, roomId, 3)
	r.TouchGameInfo("game_info")
	return pb.ErrorCode_ERR_OK
}

func (r *Role) RemovePlayRoomID(roomId uint64) pb.ErrorCode {
	info := r.PbRole.GameInfo

	if info.PlayRoomIds != nil {
		var removed bool
		info.PlayRoomIds, removed = util.Remove(info.PlayRoomIds, roomId)
		if removed {
			r.TouchGameInfo("game_info")
		}
	}

	return pb.ErrorCode_ERR_OK
}

func (r *Role) ClearPlayRoomInfo() pb.ErrorCode {
	info := r.PbRole.GameInfo

	info.PlayRoomIds = make([]uint64, 0)
	r.TouchGameInfo("game_info")
	return pb.ErrorCode_ERR_OK
}

// ============================================================
// 游戏段组件
// ============================================================

type GameComponent struct{ sectionBase }

func NewGameComponent() *GameComponent { return &GameComponent{} }

func (c *GameComponent) Name() string { return "game" }

func (c *GameComponent) Flag() pb.ERoleSectionFlag {
	return pb.ERoleSectionFlag_GAME_INFO
}

func (c *GameComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *GameComponent) OnDestroy()           {}

func (c *GameComponent) InitField(uid uint64) {
	if c.role.PbRole.GameInfo == nil {
		c.role.PbRole.GameInfo = &pb.RoleGameInfo{}
		c.role.PbRole.GameInfo.PlayRoomIds = make([]uint64, 0)
	}
}

func (c *GameComponent) Touch(reason string) {
	c.touch(pb.ERoleSectionFlag_GAME_INFO, reason)
}

func gameSection() roleSection {
	return messageSection(pb.ERoleSectionFlag_GAME_INFO, "game",
		func(i *pb.RoleInfo) *pb.RoleGameInfo { return i.GameInfo },
		func(i *pb.RoleInfo, m *pb.RoleGameInfo) { i.GameInfo = m },
		func() *pb.RoleGameInfo { return new(pb.RoleGameInfo) })
}
