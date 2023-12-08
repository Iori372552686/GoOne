package misc

import (
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	g1_protocol "github.com/Iori372552686/GoOne/protobuf/protocol"
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

// TODO 后面根据uid来判断zone
func GetZone(uid uint64) int32 {
	return 1
}

func IsRobot(uid uint64) bool {
	return uid < 100
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
func SplitItemFromArray(a []int32) *[]*g1_protocol.PbItem {
	items := make([]*g1_protocol.PbItem, 0)
	if len(a)%2 == 1 {
		return &items
	} //容错处理

	for i := 0; i < len(a); i += 2 {
		item := &g1_protocol.PbItem{}
		item.Id = a[i]
		item.Count = a[i+1]
		items = append(items, item)
	}
	return &items
}

func GetIconDescFromRoleBrief(brief *g1_protocol.PbRoleBriefInfo) *g1_protocol.PbIconDesc {
	icon := &g1_protocol.PbIconDesc{
		Uid:            brief.Uid,
		Name:           brief.Name,
		Icon:           brief.Icon,
		Frame:          brief.Frame,
		Level:          brief.Level,
		VipLevel:       brief.VipLevel,
		LastOnlineTime: brief.LastOnlineTime,
		IsOnline:       brief.LastOnlineTime+30 > datetime.Now(),
	}
	return icon
}

func GetRoleIconDesc(role *g1_protocol.RoleInfo) (iconDesc *g1_protocol.PbIconDesc) {
	iconDesc = &g1_protocol.PbIconDesc{}
	iconDesc.Level = role.BasicInfo.Level
	iconDesc.Icon = role.DescInfo.IconId
	iconDesc.Frame = role.DescInfo.FrameId
	//iconDesc.VipLevel = 0
	iconDesc.Name = role.DescInfo.Name
	return
}
