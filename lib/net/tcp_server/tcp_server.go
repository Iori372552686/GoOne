package tcp_server

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/bufpool"
	"github.com/Iori372552686/GoOne/module/misc"
)

const (
	kReadBufSize = 1024 * 10
)

type TcpConnInfo struct {
	chanWrite chan *bufpool.Buffer // passing 'nil' means close
	// closed 标记连接正在拆除。destroyConn 在锁内置 true 并删除 map；WriteData/Close
	// 在锁内读取该标志，true 时放弃入队，避免 send-on-closed-channel panic（P0-04）。
	// chanWrite 的实际 close 仍在 destroyConn 中完成，但只在确认 closed 后进行。
	closed bool
}

type TcpSvr struct {
	TcpReadTimeout  time.Duration
	TcpWriteTimeout time.Duration

	handler ITcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]*TcpConnInfo

	// listener 字段化：Quiesce 关闭 listener 停止接新连接但保留
	// 既有连接；Stop 关闭 listener 与全部连接。StopCloseOnce 保证幂等。
	listener       net.Listener
	stopCloseOnce  sync.Once
}

func (s *TcpSvr) InitAndRun(ip string, port int, handler ITcpSvrEventHandler) error {
	s.TcpReadTimeout = 2 * misc.ClientExpiryThreshold
	s.TcpWriteTimeout = 10 * time.Second

	s.handler = handler
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo = make(map[net.Conn]*TcpConnInfo)
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

// Quiesce 关闭 listener 停止接收新连接，但保留既有连接继续处理在途工作。幂等。
func (s *TcpSvr) Quiesce() {
	s.stopCloseOnce.Do(func() {
		if s.listener != nil {
			_ = s.listener.Close() // 使 runListener 的 Accept 返回 err 并退出。
		}
	})
}

// Stop 关闭 listener 与全部已建立连接，用于排空超时后的强制关停。幂等。
//
// P0-04：先在锁内快照连接列表并立即释放锁，再在锁外逐个 Close。禁止在连接表锁内
// 执行网络 Close，否则慢连接会阻塞 lookup/remove 等持锁操作。
func (s *TcpSvr) Stop() {
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
}

func (s *TcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {
	var chanWrite chan *bufpool.Buffer = nil

	s.lockOfConnInfo.RLock()
	info, exists := s.mapOfConnInfo[conn]
	if exists && !info.closed {
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
	if exists && !info.closed {
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
		s.mapOfConnInfo[conn] = &TcpConnInfo{chanWrite: chanWrite}
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
		// P0-04：socket deadline 必须用 time.Now()，不得用 datetime.NowT()（100ms tick
		// 刷新的缓存时间）。否则在 datetime tick 未运行或缓存陈旧时，deadline 会以过
		// 去时间为基准提前到期，导致读返回 i/o timeout（见 BenchmarkTCPEchoPipelined
		// 历史失败）。
		_ = conn.SetReadDeadline(time.Now().Add(s.TcpReadTimeout))
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

// destroyConn 拆除一个连接。P0-04：锁内置 closed=true 并删除；不调用 close(chanWrite)
//（直接 close channel 会与并发 WriteData 的 send 产生 send-on-closed-channel 竞态）。
// 改为向 chanWrite 投递一个 nil（关闭信号）：写协程收到 nil 即退出循环并 Close 底层
// conn。chan 本身随 entry 一起被 GC。
//
// 投递 nil 用非阻塞 select：若 chan 已满（写协程慢），跳过 nil 投递，写协程最终会
// 因 conn 已关闭（Stop/Quiesce 路径）或写超时而退出。
func (s *TcpSvr) destroyConn(conn net.Conn) {
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

		// P0-04：socket deadline 用 time.Now()，不用 datetime.NowT()。
		_ = conn.SetWriteDeadline(time.Now().Add(s.TcpWriteTimeout))
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
