package tos

import (
	"errors"
	"net"
	"os"
)

func GetLocalIp() (addr string, err error) {
	var addrList []net.Addr
	addrList, err = net.InterfaceAddrs()

	if err != nil {
		return
	}

	for _, address := range addrList {
		// 检查ip地址判断是否回环地址
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				addr = ipNet.IP.String()
				return
			}

		}
	}
	err = errors.New("Can not find the client ip address")
	return
}

func GetHostName() (name string) {
	name, _ = os.Hostname()
	return
}
