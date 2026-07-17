package ws_server

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/lib/util/bufpool"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/gorilla/websocket"
)

// 场景	ReadBufferSize	WriteBufferSize	备注
// 高频小包（动作类游戏）	4KB ~ 8KB	8KB ~ 16KB	降低内存，依赖自动扩容
// 低频大包（策略类游戏）	16KB ~ 32KB	32KB ~ 64KB	减少扩容开销
// 万级高并发	4KB	8KB	内存优先，牺牲少量性能
var upgrader = websocket.Upgrader{
	ReadBufferSize:  8 * 1024,  // 调整为自定义大小（单位：字节）
	WriteBufferSize: 16 * 1024, // 写入缓冲区同理
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TcpConnInfo struct {
	chanWrite chan *bufpool.Buffer // passing 'nil' means close
}

type WsTcpSvr struct {
	wsReadTimeout  time.Duration
	wsWriteTimeout time.Duration

	handler IWsTcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]chan *bufpool.Buffer

	// accepting 控制 wsGinPageUpgrader 是否接受新 Upgrade（roadmap P0-07）。Quiesce
	// 置 false 后新连接被拒绝，既有连接保留处理在途工作。
	accepting atomic.Bool
}

func (s *WsTcpSvr) InitAndRun(implType, mod string, port int, handler IWsTcpSvrEventHandler) error {
	s.wsReadTimeout = misc.ClientExpiryThreshold
	s.wsWriteTimeout = 5 * time.Second

	s.handler = handler
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo = make(map[net.Conn]chan *bufpool.Buffer)
	s.lockOfConnInfo.Unlock()
	s.accepting.Store(true)

	switch implType {
	case "beego":
		// 未实现的后端：显式报错，避免调用方误以为服务已启动。
		return fmt.Errorf("ws implType %q is not implemented yet, use default (gin)", implType)
	default:
		logger.Infof("init type default gin ws !")
	}

	return s.RunGinWs(mod, port)
}

// Quiesce 停止接受新 WS Upgrade，保留既有连接处理在途工作（roadmap P0-07）。幂等。
func (s *WsTcpSvr) Quiesce() {
	s.accepting.Store(false)
}

// Stop 拒绝新 Upgrade 并关闭全部已建立连接，用于排空超时后的强制关停。幂等。
func (s *WsTcpSvr) Stop() {
	s.Quiesce()
	s.lockOfConnInfo.Lock()
	for conn := range s.mapOfConnInfo {
		_ = conn.Close()
	}
	s.lockOfConnInfo.Unlock()
}

func (s *WsTcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {

	s.lockOfConnInfo.RLock()
	chanWrite := s.mapOfConnInfo[conn]
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	// 写缓冲从池中获取（Lease），由写协程在 WriteMessage 完成后归还。
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

func (s *WsTcpSvr) Close(conn net.Conn) error {
	s.lockOfConnInfo.RLock()
	chanWrite := s.mapOfConnInfo[conn]

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

func (s *WsTcpSvr) runConnRead(conn *websocket.Conn) {
	defer conn.Close()
	conn.SetReadLimit(4 * 1024 * 1024) // 单条消息最大 4MB（防止内存耗尽攻击）

	for {
		conn.SetReadDeadline(datetime.NowT().Add(s.wsReadTimeout)) // 防止慢连接占用资源
		_, data, err := conn.ReadMessage()
		//logger.Debugf("read ws type:%v  datalen: %d", dtype, len(data))

		if err == nil {
			s.handler.OnRead(conn.NetConn(), data)
		} else {
			logger.Errorf("read client[%v],msg err  | %v ", conn.RemoteAddr().String(), err)
			break
		}
	}

	s.handler.OnClose(conn.NetConn())
	s.destroyConn(conn.NetConn())
}

func (s *WsTcpSvr) destroyConn(conn net.Conn) {
	s.lockOfConnInfo.Lock()
	if s.mapOfConnInfo[conn] != nil {
		close(s.mapOfConnInfo[conn])
		delete(s.mapOfConnInfo, conn)
	}
	s.lockOfConnInfo.Unlock()
}

func (s *WsTcpSvr) runConnWrite(conn *websocket.Conn, chanWrite <-chan *bufpool.Buffer) {
	defer conn.Close()

	for {
		buf, ok := <-chanWrite
		if !ok { // chan is closed
			logger.Debugf("chanWrite is closed {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		if buf == nil { // nil means close
			logger.Infof("A 'nil' is passed to chanWrite to close conn {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		conn.SetWriteDeadline(datetime.NowT().Add(s.wsWriteTimeout))
		dataLen := len(buf.Bytes)
		err := conn.WriteMessage(websocket.BinaryMessage, buf.Bytes)
		bufpool.Release(buf) // writer 完成（含出错）后释放 Lease，只释放一次。
		if err != nil {
			logger.Errorf("Failed to write tcp data {err:%v, dataLen: %v}", err, dataLen)
			break
		}
	}
}
