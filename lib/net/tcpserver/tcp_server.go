package tcpserver

import (
	"GoOne/lib/logger"
	"bytes"
	"fmt"
	"github.com/golang/glog"
	"io"
	"net"

	"strconv"
	"sync"
	"time"
)

const (
	kReadBufSize = 1024 * 10
)

type ITcpSvrEventHandler interface {
	OnConn(net.Conn)  // 被Listener协程调用，一个TcpSvr对应一个Listener协程
	OnRead(net.Conn, []byte) int  // 被Read协程调用，每个Connection对应一个Read协调
	OnClose(net.Conn)  // 被Read协程调用，每个Connection对应一个Read协调
}

type TcpConnInfo struct {
	chanWrite chan []byte  // passing 'nil' means close
}


type TcpSvr struct {
	TcpReadTimeout time.Duration
	TcpWriteTimeout time.Duration

	handler ITcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]TcpConnInfo
}

func (s *TcpSvr) InitAndRun(ip string, port int, handler ITcpSvrEventHandler) error {
	s.TcpReadTimeout = 10 * time.Second
	s.TcpWriteTimeout = 10 * time.Second

	s.handler = handler
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo = make(map[net.Conn]TcpConnInfo)
	s.lockOfConnInfo.Unlock()

	addr := ip + ":" + strconv.Itoa(port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Errorf("Failed to listen {err=%v}:", err.Error())
		return err
	}

	logger.Infof("Listening on " + addr)
	go s.runListener(listener)
	return nil
}

func (s *TcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {
	var chanWrite chan []byte = nil

	s.lockOfConnInfo.RLock()
	info, exists := s.mapOfConnInfo[conn]
	if exists {
		chanWrite = info.chanWrite
	}
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	data := make([]byte, len(data1)+len(data2)); pos := 0
	copy(data[pos:], data1); pos += len(data1)
	copy(data[pos:], data2); pos += len(data2)

	t := time.NewTimer(3 * time.Second)
	defer t.Stop()
	select {
	case chanWrite <- data:
	case <- t.C:
		return fmt.Errorf("time out in 3 seconds")
	}

	return nil
}

func (s *TcpSvr) Close(conn net.Conn) error {
	var chanWrite chan []byte = nil

	s.lockOfConnInfo.RLock()
	info, exists := s.mapOfConnInfo[conn]
	if exists {
		chanWrite = info.chanWrite
	}
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	t := time.NewTimer(3 * time.Second)
	defer t.Stop()
	select {
	case chanWrite <- nil:
	case <- t.C:
		return fmt.Errorf("time out in 3 seconds")
	}

	return nil
}


func (s *TcpSvr) runListener(listener net.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("Error accepting: %v", err)  //todo: fatal or error
			return
		}

		chanWrite := make(chan []byte, 100)
		s.lockOfConnInfo.Lock()
		s.mapOfConnInfo[conn] = TcpConnInfo{chanWrite: chanWrite}
		s.lockOfConnInfo.Unlock()

		logger.Debugf("New Connection: {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())

		s.handler.OnConn(conn)
		go s.runConnRead(conn)
		go s.runConnWrite(conn, chanWrite)
	}
}

func (s *TcpSvr) runConnRead(conn net.Conn) {
	defer conn.Close()

	var buff bytes.Buffer
	readBuf := make([]byte, kReadBufSize)

	for {
		_ = conn.SetReadDeadline(time.Now().Add(s.TcpReadTimeout))
		readLen, err := conn.Read(readBuf)
		glog.Infof("read len: %d", readLen)

		if err == nil {
			buff.Write(readBuf[0:readLen])
			consumedLen := s.handler.OnRead(conn, buff.Bytes())
			if consumedLen > 0 {
				buff.Next(consumedLen)
			}
		} else if err == io.EOF {
			break
		} else {
			glog.Errorf("error occurs when read from tcp {errorType:%T, error:%v}", err, err)
			break
		}
	}

	s.handler.OnClose(conn)
	s.destroyConn(conn)
}

func (s *TcpSvr) destroyConn(conn net.Conn) {
	s.lockOfConnInfo.Lock()
	defer s.lockOfConnInfo.Unlock()

	if info, exists := s.mapOfConnInfo[conn]; exists {
		close(info.chanWrite)
		delete(s.mapOfConnInfo, conn)
	}
}

func (s *TcpSvr) runConnWrite(conn net.Conn, chanWrite <-chan []byte) {
	for {
		writeData, ok := <-chanWrite
		if !ok { // chan is closed
			logger.Debugf("chanWrite is closed {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		if writeData == nil {  // nil means close
			logger.Infof("A 'nil' is passed to chanWrite to close conn {local:%v, remote:%v}",
				conn.LocalAddr(), conn.RemoteAddr())
			_ = conn.Close()
			break
		}

		_ = conn.SetWriteDeadline(time.Now().Add(s.TcpWriteTimeout))
		sentLen, err := conn.Write(writeData)
		if sentLen < len(writeData) || err != nil {  //todo: retry?
			logger.Errorf("Failed to write tcp data {err:%v, dataLen: %v, sentLen: %v}",
				err, len(writeData), sentLen)
			_ = conn.Close()
			break
		}
	}
}
