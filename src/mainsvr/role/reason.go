package role

import pb "github.com/Iori372552686/game_protocol/protocol"

// 数据变化原因
type Reason struct {
	Reason pb.Reason
	Scene  int32
}
