// Package kcp_server provides the KCP transport for the gateway, aligned
// with tcp_server: per-connection read/write goroutines, a stream reassembly
// buffer for frame splitting, pooled write buffers and refreshed read
// deadlines. The *kcp.UDPSession is exposed as net.Conn so the session layer
// (net_mgr) shares its code with the TCP/WS gateways.
package kcp_server

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/bufpool"
	"github.com/Iori372552686/GoOne/module/misc"
	kcp "github.com/xtaci/kcp-go/v5"
)

const kReadBufSize = 64 * 1024

type kcpConnInfo struct {
	chanWrite chan *bufpool.Buffer // passing 'nil' means close
}

// KcpSvr is the raw-stream KCP server (mirrors tcp_server.TcpSvr).
type KcpSvr struct {
	KcpReadTimeout  time.Duration
	KcpWriteTimeout time.Duration

	handler IKcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]kcpConnInfo
}

func (s *KcpSvr) InitAndRun(ip string, port int, handler IKcpSvrEventHandler) error {
	s.KcpReadTimeout = 2 * misc.ClientExpiryThreshold
	s.KcpWriteTimeout = 10 * time.Second

	s.handler = handler
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo = make(map[net.Conn]kcpConnInfo)
	s.lockOfConnInfo.Unlock()

	addr := ip + ":" + strconv.Itoa(port)
	listener, err := kcp.ListenWithOptions(addr, nil, 0, 0)
	if err != nil {
		logger.Errorf("Failed to listen kcp {addr:%v, err:%v}", addr, err)
		return err
	}
	_ = listener.SetDSCP(46)

	logger.Infof("Listening kcp on %s", addr)
	go s.runListener(listener)
	return nil
}

// WriteData enqueues data1+data2 to the connection's write goroutine.
// The merged buffer comes from bufpool and is recycled after Write.
func (s *KcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {
	var chanWrite chan *bufpool.Buffer

	s.lockOfConnInfo.RLock()
	if info, exists := s.mapOfConnInfo[conn]; exists {
		chanWrite = info.chanWrite
	}
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("kcp connection doesn't exist")
	}

	total := len(data1) + len(data2)
	buf := bufpool.Acquire(total)
	pos := copy(buf.Bytes, data1)
	copy(buf.Bytes[pos:], data2)

	err := sendToWriteChan(chanWrite, buf)
	if err != nil {
		// 入队失败：sender 释放 Lease。
		bufpool.Release(buf)
	}
	return err
}

func (s *KcpSvr) Close(conn net.Conn) error {
	var chanWrite chan *bufpool.Buffer

	s.lockOfConnInfo.RLock()
	if info, exists := s.mapOfConnInfo[conn]; exists {
		chanWrite = info.chanWrite
	}
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("kcp connection doesn't exist")
	}

	return sendToWriteChan(chanWrite, nil)
}

func (s *KcpSvr) runListener(listener *kcp.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			logger.Errorf("Error accepting kcp: %v", err)
			return
		}

		tuneSession(conn)

		chanWrite := make(chan *bufpool.Buffer, 100)
		s.lockOfConnInfo.Lock()
		s.mapOfConnInfo[conn] = kcpConnInfo{chanWrite: chanWrite}
		s.lockOfConnInfo.Unlock()

		logger.Debugf("New kcp connection: {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())

		s.handler.OnConn(conn)
		go s.runConnRead(conn)
		go s.runConnWrite(conn, chanWrite)
	}
}

// tuneSession applies the low-latency profile used for realtime games.
func tuneSession(conn *kcp.UDPSession) {
	conn.SetStreamMode(true) // 流模式：允许分片合并，粘包由上层按帧长拆分
	conn.SetWriteDelay(false)
	conn.SetWindowSize(2048, 2048)
	conn.SetNoDelay(1, 10, 2, 1)
	_ = conn.SetDSCP(46)
	_ = conn.SetMtu(1400)
	conn.SetACKNoDelay(false)
}

func (s *KcpSvr) runConnRead(conn net.Conn) {
	defer conn.Close()

	var buff bytes.Buffer
	readBuf := make([]byte, kReadBufSize)

	for {
		_ = conn.SetReadDeadline(datetime.NowT().Add(s.KcpReadTimeout))
		readLen, err := conn.Read(readBuf)

		if err == nil {
			buff.Write(readBuf[0:readLen])
			consumedLen := s.handler.OnRead(conn, buff.Bytes())
			if consumedLen > 0 {
				buff.Next(consumedLen)
			}
		} else if err == io.EOF {
			break
		} else {
			logger.Errorf("error occurs when read from kcp {errorType:%T, error:%v}", err, err)
			break
		}
	}

	s.handler.OnClose(conn)
	s.destroyConn(conn)
}

func (s *KcpSvr) destroyConn(conn net.Conn) {
	s.lockOfConnInfo.Lock()
	if info, exists := s.mapOfConnInfo[conn]; exists {
		close(info.chanWrite)
		delete(s.mapOfConnInfo, conn)
	}
	s.lockOfConnInfo.Unlock()
}

func (s *KcpSvr) runConnWrite(conn net.Conn, chanWrite <-chan *bufpool.Buffer) {
	for {
		buf, ok := <-chanWrite
		if !ok { // chan is closed
			logger.Debugf("kcp chanWrite is closed {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		if buf == nil { // nil means close
			logger.Infof("A 'nil' is passed to kcp chanWrite to close conn {local:%v, remote:%v}",
				conn.LocalAddr(), conn.RemoteAddr())
			_ = conn.Close()
			break
		}

		_ = conn.SetWriteDeadline(datetime.NowT().Add(s.KcpWriteTimeout))
		sentLen, err := conn.Write(buf.Bytes)
		dataLen := len(buf.Bytes)
		bufpool.Release(buf) // writer 完成（含出错）后释放 Lease，只释放一次。
		if sentLen < dataLen || err != nil {
			logger.Errorf("Failed to write kcp data {err:%v, dataLen: %v, sentLen: %v}", err, dataLen, sentLen)
			_ = conn.Close()
			break
		}
	}
}

// sendToWriteChan enqueues a buffer lease (nil means close) with a bounded wait.
// The fast path avoids the timer allocation entirely.
func sendToWriteChan(chanWrite chan *bufpool.Buffer, buf *bufpool.Buffer) error {
	select {
	case chanWrite <- buf:
		return nil
	default:
	}

	t := time.NewTimer(3 * time.Second)
	defer t.Stop()
	select {
	case chanWrite <- buf:
		return nil
	case <-t.C:
		return fmt.Errorf("time out in 3 seconds")
	}
}
