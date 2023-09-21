package util

import (
	"fmt"
	"strconv"
	"strings"
)

/**
* @Description:
* @param: version
* @return: int
* @Author: Iori
* @Date: 2022-07-05 20:12:53
**/

func VersionToNum(version string) uint32 {
	arrStr := strings.Split(version, ".")
	var ipInt uint32 = 0
	var pos uint = 8

	for _, ipSeg := range arrStr {
		tempInt, _ := strconv.Atoi(ipSeg)
		ipInt = (ipInt << pos) | uint32(tempInt)
	}
	return ipInt
}

/**
* @Description:
* @param: num
* @return: string
* @Author: Iori
* @Date: 2022-07-05 20:12:55
**/
func NumToVersion(num int) string {
	mainV := num >> 16
	subV := ((0xff << 8) & num) >> 8
	minV := ((0xff << 0) & num) >> 0

	return fmt.Sprintf("%d.%d.%d", mainV, subV, minV)
}
