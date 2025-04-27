/// 玩家邮箱，相框，立绘相关

package role

import (
	"fmt"
	g1_protocol "github.com/Iori372552686/game_protocol"
)

func (r *Role) GetIconDesc() *g1_protocol.PbIconDesc {
	desc := &g1_protocol.PbIconDesc{}
	desc.Name = r.PbRole.BasicInfo.Name
	desc.IconUrl = r.PbRole.IconInfo.IconUrl
	desc.Frame = r.PbRole.IconInfo.FrameId
	desc.Level = r.PbRole.BasicInfo.Level
	desc.IsOnline = r.IsOnline()
	desc.Uid = r.PbRole.RegisterInfo.Uid
	//desc.VipLevel = 0
	return desc
}

func (r *Role) IconGet(id int32, addIfExist bool) *g1_protocol.PbIcon {
	if r.PbRole.IconInfo.IconMap == nil {
		r.PbRole.IconInfo.IconMap = make(map[int32]*g1_protocol.PbIcon)
	}

	if r.PbRole.IconInfo.IconMap[id] != nil {
		return r.PbRole.IconInfo.IconMap[id]
	}

	if addIfExist {
		ins := &g1_protocol.PbIcon{Id: id}
		r.PbRole.IconInfo.IconMap[id] = ins
		return ins
	}
	return nil
}

func (r *Role) FrameGet(id int32, addIfExist bool) *g1_protocol.PbFrame {
	if r.PbRole.IconInfo.FrameMap == nil {
		r.PbRole.IconInfo.FrameMap = make(map[int32]*g1_protocol.PbFrame)
	}

	if r.PbRole.IconInfo.FrameMap[id] != nil {
		return r.PbRole.IconInfo.FrameMap[id]
	}

	if addIfExist {
		ins := &g1_protocol.PbFrame{Id: id}
		r.PbRole.IconInfo.FrameMap[id] = ins
		return ins
	}
	return nil
}

func (r *Role) IconAdd(id int32, reason *Reason) int {
	icon := r.IconGet(id, true)
	if reason.Reason != g1_protocol.Reason_REASON_INIT {
		icon.RedPoint = true
	}
	return 0
}

func (r *Role) IconHas(id int32) bool {
	v := r.IconGet(id, false)
	return v != nil
}

func (r *Role) FrameAdd(id int32, reason *Reason) int {
	frame := r.FrameGet(id, true)
	if reason.Reason != g1_protocol.Reason_REASON_INIT {
		frame.RedPoint = true
	}
	return 0
}

func (r *Role) FrameHas(id int32) bool {
	v := r.FrameGet(id, false)
	return v != nil
}

func (r *Role) IconChange(iconId int32) g1_protocol.ErrorCode {
	if iconId <= 0 { //&& !r.IconHas(iconId)
		return g1_protocol.ErrorCode_ERR_ICON_NOT_HAVE
	}
	r.PbRole.IconInfo.IconUrl = fmt.Sprintf("headicon_%d", iconId)
	return g1_protocol.ErrorCode_ERR_OK
}

func (r *Role) FrameChange(frameId int32) g1_protocol.ErrorCode {
	if frameId > 0 && !r.FrameHas(frameId) {
		return g1_protocol.ErrorCode_ERR_FRAME_NOT_HAVE
	}
	r.PbRole.IconInfo.FrameId = frameId
	return g1_protocol.ErrorCode_ERR_OK
}

func (r *Role) ImageChange(imageId int32) g1_protocol.ErrorCode {

	return g1_protocol.ErrorCode_ERR_OK
}

func (r *Role) IconTouchRedPoint(id int32) int {
	icon := r.IconGet(id, false)
	if icon != nil {
		icon.RedPoint = false
	}
	return 0
}

func (r *Role) FrameTouchRedPoint(id int32) int {
	frame := r.FrameGet(id, false)
	if frame != nil {
		frame.RedPoint = false
	}
	return 0
}
