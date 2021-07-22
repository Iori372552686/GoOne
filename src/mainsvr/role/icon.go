/// 玩家邮箱，相框，立绘相关


package role

import (
	g1_protocol `GoOne/protobuf/protocol`
)

func (r *Role) IconGetIconDesc() *g1_protocol.PbIconDesc {
	desc := &g1_protocol.PbIconDesc{}
	desc.Name = r.PbRole.DescInfo.Name
	desc.Icon = r.PbRole.DescInfo.IconId
	desc.Frame = r.PbRole.DescInfo.FrameId
	desc.Level = r.PbRole.BasicInfo.Level
	//desc.VipLevel = 0
	return desc
}


func (r *Role) IconGet(id int32, addIfExist bool) *g1_protocol.PbIcon {
	if r.PbRole.IconInfo.IconList == nil {
		r.PbRole.IconInfo.IconList = make([]*g1_protocol.PbIcon, 0)
	}
	for _, v := range r.PbRole.IconInfo.IconList {
		if v.Id == id {
			return v
		}
	}

	if addIfExist {
		v := &g1_protocol.PbIcon{Id: id}
		r.PbRole.IconInfo.IconList = append(r.PbRole.IconInfo.IconList, v)
		return v
	}
	return nil
}

func (r *Role) FrameGet(id int32, addIfExist bool) *g1_protocol.PbFrame {
	if r.PbRole.IconInfo.FrameList == nil {
		r.PbRole.IconInfo.FrameList = make([]*g1_protocol.PbFrame, 0)
	}
	for _, v := range r.PbRole.IconInfo.FrameList {
		if v.Id == id {
			return v
		}
	}

	if addIfExist {
		v := &g1_protocol.PbFrame{Id: id}
		r.PbRole.IconInfo.FrameList = append(r.PbRole.IconInfo.FrameList, v)
		return v
	}
	return nil
}

func (r *Role) IconAdd(id int32, reason *Reason) int {
	icon := r.IconGet(id, true)
	if reason.Reason != int32(REASON_INIT) {
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
	if reason.Reason != int32(REASON_INIT) {
		frame.RedPoint = true
	}
	return 0
}

func (r *Role) FrameHas(id int32) bool {
	v := r.FrameGet(id, false)
	return v != nil
}

func (r *Role) IconChange(iconId int32) int {
	if iconId > 0 && !r.IconHas(iconId) {
		return int(g1_protocol.ErrorCode_ERR_ICON_NOT_HAVE)
	}
	r.PbRole.DescInfo.IconId = iconId
	return 0
}

func (r *Role) FrameChange(frameId int32) int {
	if frameId > 0 && !r.FrameHas(frameId) {
		return int(g1_protocol.ErrorCode_ERR_FRAME_NOT_HAVE)
	}
	r.PbRole.DescInfo.FrameId = frameId
	return 0
}

func (r *Role) ImageChange(imageId int32) int {

	return 0
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



