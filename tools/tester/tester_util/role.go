package tester_util

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	g1_protocol "github.com/Iori372552686/game_protocol/protocol"
	"github.com/golang/protobuf/proto"
)

type Role struct {
	uid uint64
}

func (r *Role) SetUid(uid uint64) {
	r.uid = uid
}

func (r *Role) GetUid() uint64 {
	return r.uid
}

func (r *Role) Getlogin() ([]byte, error) {
	body := &g1_protocol.LoginReq{
		Account:   "test",
		Token:     "ykocG0rfZkgWDY07i8%2FiKdlKCrU5x0T07BinE%2FIRPpSG0R4JsvwpjnM7TYOHIhBlaWk%2BQnBj%2FMbNdUETZJVwU8nIRRfWkTO%2Bcdek4lc8HpLYMGjTUpBG4l2vA3Gkw9KoRp7DCF3ViOCgvR9t%2FFySQs54JGKhnDItWNdlcNKeVX0a6EmqR1Zm%2FmrswYgLGqrf",
		ChannelId: 1,
		LoginType: "guest", //游客登陆
		DeviceOs:  "web",
	}

	bodyBytes, err := proto.Marshal(body)
	if err != nil {
		return nil, err
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      0,
		Cmd:      uint32(g1_protocol.CMD_MAIN_LOGIN_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}

	return append(header.ToBytes(), bodyBytes...), nil
}

func (r *Role) GetHeartbeat(uid uint64) ([]byte, error) {
	timeMs := datetime.NowMs()
	body := &g1_protocol.HeartBeatReq{
		ClientNowMs: timeMs,
	}
	bodyBytes, err := proto.Marshal(body)

	if err != nil {
		fmt.Printf("heartBeat Marshal error: %v", err)
		return nil, err
	}

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      uid,
		Cmd:      uint32(g1_protocol.CMD_MAIN_HEARTBEAT_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}
	return append(header.ToBytes(), bodyBytes...), nil
}
