package kcp_server

import (
	"GoMini/common/misc"
	"GoMini/lib/api/logger"
	"io"
	"strconv"
	"sync"
	"time"

	Kcp "github.com/xtaci/kcp-go/v5"
)

const (
	kReadBufSize = 65536
	kcpSockBuf   = 128 * 1024 * 1024
)

/*
*  KcpSvr
*  @Description:
 */
type KcpSvr struct {
	KcpReadTimeout  time.Duration
	kcpWriteTimeout time.Duration

	handler IKcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[*Kcp.UDPSession]chan []byte
}

/**
* @Description: init
* @receiver: self
* @param: port
* @param: handler
* @return: error
* @Author: Iori
* @Date: 2022-02-15 14:36:19
**/
func (self *KcpSvr) InitAndRun(port int, handler IKcpSvrEventHandler) error {
	self.KcpReadTimeout = 2 * misc.ClientExpiryThreshold
	self.kcpWriteTimeout = 10 * time.Second
	self.handler = handler
	self.mapOfConnInfo = make(map[*Kcp.UDPSession]chan []byte)

	//no block
	if listener, err := Kcp.ListenWithOptions("0.0.0.0:"+strconv.Itoa(port), nil, 0, 0); err == nil {
		go self.runListener(listener)
	} else {
		return err
	}

	return nil
}

/**
* @Description:
* @receiver: self
* @param: listener
* @Author: Iori
* @Date: 2022-02-15 14:47:37
**/
func (self *KcpSvr) runListener(listener *Kcp.Listener) {
	defer listener.Close()
	listener.SetDSCP(46)
	//listener.SetReadBuffer(kcpSockBuf)
	//listener.SetWriteBuffer(kcpSockBuf)

	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			logger.Errorf(err.Error())
		}

		chanWrite := make(chan []byte, 100)
		self.lockOfConnInfo.Lock()
		self.mapOfConnInfo[conn] = chanWrite
		self.lockOfConnInfo.Unlock()
		//logger.Debugf("New Kcp Connection: {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())

		self.handler.OnConn(conn)
		go self.runConnRead(conn)
		go self.runConnWrite(conn, chanWrite)
	}
}

/**
* @Description:
* @receiver: self
* @param: conn
* @Author: Iori
* @Date: 2022-02-15 14:47:40
**/
func (self *KcpSvr) runConnRead(conn *Kcp.UDPSession) {
	defer conn.Close()
	readBuf := make([]byte, kReadBufSize)

	//kcp opt
	//conn.SetStreamMode(true)
	conn.SetWriteDelay(false)
	conn.SetWindowSize(2048, 2048)
	conn.SetNoDelay(1, 10, 2, 1)
	conn.SetDSCP(46)
	conn.SetMtu(1400)
	conn.SetACKNoDelay(false)
	conn.SetReadDeadline(time.Now().Add(time.Hour))
	conn.SetWriteDeadline(time.Now().Add(time.Hour))

	for {
		readLen, err := conn.Read(readBuf)
		if err == nil {
			self.handler.OnRead(conn, readBuf[:readLen])
		} else if err == io.EOF {
			break
		} else {
			logger.Errorf("error occurs when read from kcp {errorType:%T, error:%v}", err, err)
			break
		}
	}

	self.handler.OnClose(conn)
	self.destroyConn(conn)
}

/**
* @Description:
* @receiver: self
* @param: conn
* @Author: Iori
* @Date: 2022-02-15 14:47:44
**/
func (self *KcpSvr) destroyConn(conn *Kcp.UDPSession) {
	self.lockOfConnInfo.Lock()
	defer self.lockOfConnInfo.Unlock()

	if chanWrite, exists := self.mapOfConnInfo[conn]; exists {
		close(chanWrite)
		delete(self.mapOfConnInfo, conn)
	}
}

/**
* @Description:
* @receiver: self
* @param: conn
* @param: chanWrite
* @Author: Iori
* @Date: 2022-02-15 14:47:48
**/
func (self *KcpSvr) runConnWrite(conn *Kcp.UDPSession, chanWrite <-chan []byte) {
	for {
		writeData, ok := <-chanWrite
		if !ok { // chan is closed
			logger.Debugf("Kcp chanWrite is closed {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		if writeData == nil { // nil means close
			logger.Infof("Kcp A 'nil' is passed to chanWrite to close conn {local:%v, remote:%v}",
				conn.LocalAddr(), conn.RemoteAddr())
			_ = conn.Close()
			break
		}

		_ = conn.SetWriteDeadline(time.Now().Add(self.kcpWriteTimeout))
		sentLen, err := conn.Write(writeData)
		if sentLen < len(writeData) || err != nil {
			logger.Errorf("Failed to write Kcp data {err:%v, dataLen: %v, sentLen: %v}",
				err, len(writeData), sentLen)
			_ = conn.Close()
			break
		}
	}
}
