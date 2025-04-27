package bus

import (
	"bytes"
	"strconv"
	"strings"
)

func IpStringToInt(ipstring string) uint32 {
	ipSegs := strings.Split(ipstring, ".")
	var ipInt uint32 = 0
	var pos uint = 24
	for _, ipSeg := range ipSegs {
		tempInt, _ := strconv.Atoi(ipSeg)
		tempInt = tempInt << pos
		ipInt = ipInt | uint32(tempInt)
		pos -= 8
	}
	return ipInt
}

func IpIntToString(ipInt uint32) string {
	ipSegs := make([]string, 4)
	var len int = len(ipSegs)
	buffer := bytes.NewBufferString("")
	for i := 0; i < len; i++ {
		tempInt := ipInt & 0xFF
		ipSegs[len-i-1] = strconv.Itoa(int(tempInt))
		ipInt = ipInt >> 8
	}
	for i := 0; i < len; i++ {
		buffer.WriteString(ipSegs[i])
		if i < len-1 {
			buffer.WriteString(".")
		}
	}
	return buffer.String()
}

func ParseBusID(ipstring string) (uint32, uint32, uint32, uint32, uint32) {
	ipSegs := strings.Split(ipstring, ".")
	if len(ipSegs) < 4 {
		return 0, 0, 0, 0, 0
	}

	worldID, _ := strconv.Atoi(ipSegs[0])
	zoneID, _ := strconv.Atoi(ipSegs[1])
	funcID, _ := strconv.Atoi(ipSegs[2])
	insID, _ := strconv.Atoi(ipSegs[3])

	ip := (worldID << 24) + (zoneID << 16) + (funcID << 8) + insID

	return uint32(ip), uint32(worldID), uint32(zoneID), uint32(funcID), uint32(insID)
}
