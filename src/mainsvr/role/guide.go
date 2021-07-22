package role

import (
	g1_protocol `GoOne/protobuf/protocol`
)

func (r *Role) GuideCompleted(id int32) int {
	// 是否已经存在
	for _, v := range r.PbRole.GuideInfo.IdLis {
		if v == id {
			return int(g1_protocol.ErrorCode_ERR_GUIDE_IS_EXIST)
		}
	}
	r.PbRole.GuideInfo.IdLis = append(r.PbRole.GuideInfo.IdLis, id)
	return 0
}

func (r *Role) GuideInProgress(id int32) int {
	r.PbRole.GuideInfo.CurId = id
	return 0
}
