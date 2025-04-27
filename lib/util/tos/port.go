package tos

import (
	"net"
)

func GetFreePort() (*net.TCPListener, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return nil, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func GetFreePorts(count int) ([]*net.TCPListener, error) {
	var ls []*net.TCPListener
	for i := 0; i < count; i++ {
		addr, err := net.ResolveTCPAddr("tcp", ":0")
		if err != nil {
			return nil, err
		}
		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, err
		}
		ls = append(ls, l)
	}
	return ls, nil
}
