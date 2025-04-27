package tos

import (
	"log"
	"testing"
)

//单元测试
func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		log.Println(err)
	}
	log.Println(port.Addr().String())
}

//单元测试
func TestGetFreePorts(t *testing.T) {
	port, err := GetFreePorts(3)
	if err != nil {
		log.Println(err)
	}
	for _, addr := range port {
		log.Println(addr.Addr().String())
	}

}
