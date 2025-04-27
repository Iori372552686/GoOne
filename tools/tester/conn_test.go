package tester

import (
	"github.com/Iori372552686/GoOne/lib/api/sharedstruct"
	"github.com/Iori372552686/GoOne/tools/tester/tester_util"
	g1_protocol "github.com/Iori372552686/game_protocol"
	"github.com/golang/protobuf/proto"
	"testing"
)

func TestConn(t *testing.T) {
	s := tester_util.NewSession(t)
	err := s.Open()
	if err != nil {
		return
	}
	defer s.Close()
	body := &g1_protocol.LoginReq{
		Account:   "test",
		Token:     "ykocG0rfZkgWDY07i8%2FiKdlKCrU5x0T07BinE%2FIRPpSG0R4JsvwpjnM7TYOHIhBlaWk%2BQnBj%2FMbNdUETZJVwU8nIRRfWkTO%2Bcdek4lc8HpLYMGjTUpBG4l2vA3Gkw9KoRp7DCF3ViOCgvR9t%2FFySQs54JGKhnDItWNdlcNKeVX0a6EmqR1Zm%2FmrswYgLGqrf",
		ChannelId: 1,
		LoginType: "guest",
		DeviceOs:  "web",
	}

	bodyBytes, err := proto.Marshal(body)

	header := sharedstruct.CSPacketHeader{
		Version:  1,
		PassCode: 1,
		Seq:      1,
		Uid:      0,
		Cmd:      uint32(g1_protocol.CMD_MAIN_LOGIN_REQ),
		BodyLen:  uint32(len(bodyBytes)),
	}
	s.Send(header.ToBytes())
	s.Send(bodyBytes)
}
