package tcp_server

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
)

const (
	kReadBufSize = 1024 * 10
)

type TcpConnInfo struct {
	chanWrite chan *bufpool.Buffer // passing 'nil' means close
}

type TcpSvr struct {
	TcpReadTimeout  time.Duration
	TcpWriteTimeout time.Duration

	handler ITcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]TcpConnInfo

	// listener 字段化（roadmap P0-07）：Quiesce 关闭 listener 停止接新连接但保留
	// 既有连接；Stop 关闭 listener 与全部连接。StopCloseOnce 保证幂等。
	listener       net.Listener
	stopCloseOnce  sync.Once
}

func (s *TcpSvr) InitAndRun(ip string, port int, handler ITcpSvrEventHandler) error {
	s.TcpReadTimeout = 2 * misc.ClientExpiryThreshold
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
	s.listener = listener

	logger.Infof("Listening on " + addr)
	go s.runListener(listener)
	return nil
}

// Quiesce 关闭 listener 停止接收新连接，但保留既有连接继续处理在途工作（roadmap
// P0-07）。幂等。
func (s *TcpSvr) Quiesce() {
	s.stopCloseOnce.Do(func() {
		if s.listener != nil {
			_ = s.listener.Close() // 使 runListener 的 Accept 返回 err 并退出。
		}
	})
}

// Stop 关闭 listener 与全部已建立连接，用于排空超时后的强制关停。幂等。
func (s *TcpSvr) Stop() {
	s.Quiesce()
	s.lockOfConnInfo.Lock()
	for conn := range s.mapOfConnInfo {
		_ = conn.Close()
	}
	s.lockOfConnInfo.Unlock()
}

func (s *TcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {
	var chanWrite chan *bufpool.Buffer = nil

	s.lockOfConnInfo.RLock()
	info, exists := s.mapOfConnInfo[conn]
	if exists {
		chanWrite = info.chanWrite
	}
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	// 写缓冲从池中获取（Lease），由写协程在 conn.Write 完成后归还。
	total := len(data1) + len(data2)
	buf := bufpool.Acquire(total)
	pos := 0
	copy(buf.Bytes[pos:], data1)
	pos += len(data1)
	copy(buf.Bytes[pos:], data2)
	pos += len(data2)

	err := sendToWriteChan(chanWrite, buf)
	if err != nil {
		// 入队失败：sender 释放 Lease。
		bufpool.Release(buf)
	}
	return err
}

func (s *TcpSvr) Close(conn net.Conn) error {
	var chanWrite chan *bufpool.Buffer = nil

	s.lockOfConnInfo.RLock()
	info, exists := s.mapOfConnInfo[conn]
	if exists {
		chanWrite = info.chanWrite
	}
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	return sendToWriteChan(chanWrite, nil)
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

func (s *TcpSvr) runListener(listener net.Listener) {
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Errorf("Error accepting: %v", err)
			return
		}

		chanWrite := make(chan *bufpool.Buffer, 100)
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
		_ = conn.SetReadDeadline(datetime.NowT().Add(s.TcpReadTimeout))
		readLen, err := conn.Read(readBuf)

		if err == nil {
			buff.Write(readBuf[0:readLen])
			consumedLen := s.handler.OnRead(conn, buff.Bytes())
			//consumedLen := s.handler.OnRead2(conn, buff.Bytes())
			if consumedLen > 0 {
				buff.Next(consumedLen)
			}
		} else if err == io.EOF {
			break
		} else {
			logger.Errorf("error occurs when read from tcp {errorType:%T, error:%v}", err, err)
			break
		}
	}

	s.handler.OnClose(conn)
	s.destroyConn(conn)
}

func (s *TcpSvr) destroyConn(conn net.Conn) {
	s.lockOfConnInfo.Lock()
	if info, exists := s.mapOfConnInfo[conn]; exists {
		close(info.chanWrite)
		delete(s.mapOfConnInfo, conn)
	}
	s.lockOfConnInfo.Unlock()
}

func (s *TcpSvr) runConnWrite(conn net.Conn, chanWrite <-chan *bufpool.Buffer) {
	for {
		buf, ok := <-chanWrite
		if !ok { // chan is closed
			logger.Debugf("chanWrite is closed {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		if buf == nil { // nil means close
			logger.Infof("A 'nil' is passed to chanWrite to close conn {local:%v, remote:%v}",
				conn.LocalAddr(), conn.RemoteAddr())
			_ = conn.Close()
			break
		}

		_ = conn.SetWriteDeadline(datetime.NowT().Add(s.TcpWriteTimeout))
		sentLen, err := conn.Write(buf.Bytes)
		dataLen := len(buf.Bytes)
		bufpool.Release(buf) // writer 完成（含出错）后释放 Lease，只释放一次。
		if sentLen < dataLen || err != nil { //todo: retry?
			logger.Errorf("Failed to write tcp data {err:%v, dataLen: %v, sentLen: %v}", err, dataLen, sentLen)
			_ = conn.Close()
			break
		}
	}
}
