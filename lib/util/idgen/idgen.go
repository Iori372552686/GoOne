package idgen

import (
	"github.com/Iori372552686/GoOne/lib/util/convert"
	"github.com/Iori372552686/GoOne/lib/util/tos"
	"github.com/sony/sonyflake"
	"strings"
	"time"
)

type TIDGen struct {
	idGen *sonyflake.Sonyflake
}

func NewIDGen() (idg *TIDGen, err error) {
	addr, err := tos.GetLocalIp()
	if err != nil {
		return
	}
	var mid uint16 = 1
	ss := strings.Split(addr, ".")
	if len(ss) > 0 {
		mid = uint16(convert.StrToInt(ss[len(ss)-1]))
	}
	idg = &TIDGen{
		idGen: newSonyFlake(mid),
	}
	return
}

func (i *TIDGen) GenID() (uint64, error) {
	return i.idGen.NextID()
}

func newSonyFlake(MachineID uint16) *sonyflake.Sonyflake {
	return sonyflake.NewSonyflake(sonyflake.Settings{
		StartTime: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		MachineID: func() (uint16, error) {
			return MachineID, nil
		},
		CheckMachineID: nil,
	})
}
