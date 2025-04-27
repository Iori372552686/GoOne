package misc

import (
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	g1protocol "github.com/gdsgog/poker_protocol/protocol"
)

// 从命令字中获取目标svr
func ServerTypeInCmd(cmd uint32) uint32 {
	return cmd >> 16 & 0xff
}

// 从命令字中获取命令字类型
func MsgTypeInCmd(cmd uint32) uint32 {
	return cmd >> 12 & 0xf
}

// 判断命令字时候是客户端发来
func IsClientCmd(cmd uint32) bool {
	t := MsgTypeInCmd(cmd)
	return t == 0 || t == 3
}

// 判断命令字是否为内部命令
func IsInnerCmd(cmd uint32) bool {
	return !IsClientCmd(cmd)
}

// 判断命令字是否是GM命令
func IsGmCmd(cmd uint32) bool {
	t := MsgTypeInCmd(cmd)
	return t == 0xa
}

func IsRobot(uid uint64) bool {
	return uid < 100000
}

//func SplitItemFromString(s string) *[]*g1_protocol.PbItem {
//	items := make([]*g1_protocol.PbItem, 0)
//	s1 := strings.Split(s, "|")
//	for _, v := range s1 {
//		item := &g1_protocol.PbItem{}
//		s2 := strings.Split(v, ",")
//		id, _ := strconv.Atoi(s2[0])
//		count, _ := strconv.Atoi(s2[1])
//		item.Id = int32(id)
//		item.Count = int32(count)
//		items = append(items, item)
//	}
//	return &items
//}

// 这里一般是一个id，接一个数量
func SplitItemFromArray(a []int32) *[]*g1protocol.PbItem {
	items := make([]*g1protocol.PbItem, 0)
	if len(a)%2 == 1 {
		return &items
	} //容错处理

	for i := 0; i < len(a); i += 2 {
		item := &g1protocol.PbItem{}
		item.Id = a[i]
		item.Count = int64(a[i+1])
		items = append(items, item)
	}
	return &items
}

func GetIconDescFromRoleBrief(brief *g1protocol.PbRoleBriefInfo) *g1protocol.PbIconDesc {
	icon := &g1protocol.PbIconDesc{
		Uid:            brief.Uid,
		Name:           brief.Name,
		IconUrl:        brief.IconUrl,
		Frame:          brief.Frame,
		Level:          brief.Level,
		VipLevel:       brief.VipLevel,
		LastOnlineTime: brief.LastOnlineTime,
		IsOnline:       brief.LastOnlineTime+30 > datetime.Now(),
	}
	return icon
}

func GetRoleIconDesc(role *g1protocol.RoleInfo) (iconDesc *g1protocol.PbIconDesc) {
	iconDesc = &g1protocol.PbIconDesc{}
	iconDesc.Level = role.BasicInfo.Level
	iconDesc.IconUrl = role.IconInfo.IconUrl
	iconDesc.Frame = role.IconInfo.FrameId
	//iconDesc.VipLevel = 0
	iconDesc.Name = role.BasicInfo.Name
	return
}
