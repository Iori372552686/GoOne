package gnet_svr

import (
	"fmt"
	"log"

	"github.com/panjf2000/gnet"
)

const (
	kReadBufSize = 128 * 1024 * 1024
)

type udpServer struct {
	*gnet.EventServer

	handler func(conn gnet.Conn, data []byte)
}

func (self *udpServer) OnInitComplete(srv gnet.Server) (action gnet.Action) {
	log.Printf(" Gnet UDP server is listening on %s (multi-cores: %t, loops: %d)\n",
		srv.Addr.String(), srv.Multicore, srv.NumEventLoop)
	return
}

func (self *udpServer) React(frame []byte, c gnet.Conn) (out []byte, action gnet.Action) {
	self.handler(c, frame)

	/*
		// Echo asynchronously.
		data := append([]byte{}, frame...)
		go func() {
			time.Sleep(time.Second)
			c.SendTo(data)
		}()
		return
	*/

	return
}

func NewUdpServer(port int, cb func(conn gnet.Conn, data []byte)) error {
	udp := new(udpServer)
	udp.handler = cb

	go gnet.Serve(udp, fmt.Sprintf("udp://:%d", port), gnet.WithSocketRecvBuffer(kReadBufSize), gnet.WithMulticore(true), gnet.WithReusePort(true))
	return nil
}
