// Package kcp_server provides the KCP transport for the gateway, aligned
// with tcp_server: per-connection read/write goroutines, a stream reassembly
// buffer for frame splitting, pooled write buffers and refreshed read
// deadlines. The *kcp.UDPSession is exposed as net.Conn so the session layer
// (net_mgr) shares its code with the TCP/WS gateways.
package kcp_server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/bufpool"
	"github.com/Iori372552686/GoOne/module/misc"
	kcp "github.com/xtaci/kcp-go/v5"
)

const kReadBufSize = 64 * 1024

// kcpConnInfo 封装每连接的写 channel 与关闭标志（P0-04：消除 send-on-closed-channel
// 竞态）。
type kcpConnInfo struct {
	chanWrite chan *bufpool.Buffer // passing 'nil' means close
	closed    bool                 // destroyConn 在锁内置 true，配合 WriteData/Close 检查
}

// KcpSvr is the raw-stream KCP server (mirrors tcp_server.TcpSvr).
type KcpSvr struct {
	KcpReadTimeout  time.Duration
	KcpWriteTimeout time.Duration

	handler IKcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]*kcpConnInfo

	// listener 字段化：Quiesce 关 listener 停接新连接但保留既有；
	// Stop 关 listener 与全部连接。stopCloseOnce 保证幂等。
	listener      *kcp.Listener
	stopCloseOnce sync.Once
}

func (s *KcpSvr) InitAndRun(ip string, port int, handler IKcpSvrEventHandler) error {
	s.KcpReadTimeout = 2 * misc.ClientExpiryThreshold
	s.KcpWriteTimeout = 10 * time.Second

	s.handler = handler
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo = make(map[net.Conn]*kcpConnInfo)
	s.lockOfConnInfo.Unlock()

	addr := ip + ":" + strconv.Itoa(port)
	listener, err := kcp.ListenWithOptions(addr, nil, 0, 0)
	if err != nil {
		logger.Errorf("Failed to listen kcp {addr:%v, err:%v}", addr, err)
		return err
	}
	s.listener = listener
	_ = listener.SetDSCP(46)

	logger.Infof("Listening kcp on %s", addr)
	go s.runListener(listener)
	return nil
}

// Quiesce 关闭 listener 停止接收新连接，保留既有连接处理在途工作。
// 幂等。
func (s *KcpSvr) Quiesce() {
	s.stopCloseOnce.Do(func() {
		if s.listener != nil {
			_ = s.listener.Close() // 使 runListener 的 AcceptKCP 返回 err 并退出。
		}
	})
}

// Stop 关闭 listener 与全部已建立连接，用于排空超时后的强制关停。幂等。
//
// P0-04：先在锁内快照连接列表并释放锁，再在锁外逐个 Close。禁止在连接表锁内执行
// 网络 Close。
//
// ctx 当前未使用（关闭是同步的），保留作为未来受 context 约束强制关闭的扩展点；
// 返回 error 使关闭结果可观测。
func (s *KcpSvr) Stop(_ context.Context) error {
	s.Quiesce()
	s.lockOfConnInfo.Lock()
	conns := make([]net.Conn, 0, len(s.mapOfConnInfo))
	for conn := range s.mapOfConnInfo {
		conns = append(conns, conn)
	}
	s.lockOfConnInfo.Unlock()
	for _, conn := range conns {
		_ = conn.Close()
	}
	return nil
}

// WriteData enqueues data1+data2 to the connection's write goroutine.
// The merged buffer comes from bufpool and is recycled after Write.
func (s *KcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {
	var chanWrite chan *bufpool.Buffer

	s.lockOfConnInfo.RLock()
	if info, exists := s.mapOfConnInfo[conn]; exists && !info.closed {
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
	if info, exists := s.mapOfConnInfo[conn]; exists && !info.closed {
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
		s.mapOfConnInfo[conn] = &kcpConnInfo{chanWrite: chanWrite}
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
		// P0-04：socket deadline 用 time.Now()，不用 datetime.NowT()（缓存时间会陈旧）。
		_ = conn.SetReadDeadline(time.Now().Add(s.KcpReadTimeout))
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

// destroyConn 拆除一个连接。P0-04：锁内置 closed=true 并删除；不调用 close(chanWrite)
//（直接 close channel 会与并发 WriteData 的 send 产生 send-on-closed-channel 竞态）。
// 改为向 chanWrite 投递一个 nil（关闭信号）：写协程收到 nil 即退出循环并 Close 底层
// conn。chan 本身随 entry 一起被 GC。
func (s *KcpSvr) destroyConn(conn net.Conn) {
	s.lockOfConnInfo.Lock()
	info, exists := s.mapOfConnInfo[conn]
	if exists {
		info.closed = true
		delete(s.mapOfConnInfo, conn)
	}
	s.lockOfConnInfo.Unlock()
	if exists {
		select {
		case info.chanWrite <- nil:
		default:
		}
	}
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

		// P0-04：socket deadline 用 time.Now()。
		_ = conn.SetWriteDeadline(time.Now().Add(s.KcpWriteTimeout))
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
