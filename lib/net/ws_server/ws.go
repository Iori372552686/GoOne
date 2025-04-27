package ws_server

import (
	"fmt"
	"github.com/Iori372552686/GoOne/lib/api/datetime"
	"github.com/Iori372552686/GoOne/lib/api/logger"
	"github.com/Iori372552686/GoOne/module/misc"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"sync"
	"time"
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
	chanWrite chan []byte // passing 'nil' means close
}

type WsTcpSvr struct {
	wsReadTimeout  time.Duration
	wsWriteTimeout time.Duration

	handler IWsTcpSvrEventHandler

	lockOfConnInfo sync.RWMutex
	mapOfConnInfo  map[net.Conn]chan []byte
}

func (s *WsTcpSvr) InitAndRun(implType, mod string, port int, handler IWsTcpSvrEventHandler) error {
	s.wsReadTimeout = misc.ClientExpiryThreshold
	s.wsWriteTimeout = 5 * time.Second

	s.handler = handler
	s.lockOfConnInfo.Lock()
	s.mapOfConnInfo = make(map[net.Conn]chan []byte)
	s.lockOfConnInfo.Unlock()

	switch implType {
	case "beego": //todo
	default:
		logger.Infof("init type default gin ws !")
	}

	return s.RunGinWs(mod, port)
}

func (s *WsTcpSvr) WriteData(conn net.Conn, data1 []byte, data2 []byte) error {

	s.lockOfConnInfo.RLock()
	chanWrite := s.mapOfConnInfo[conn]
	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	data := make([]byte, len(data1)+len(data2))
	pos := 0
	copy(data[pos:], data1)
	pos += len(data1)
	copy(data[pos:], data2)
	pos += len(data2)

	t := time.NewTimer(3 * time.Second)
	defer t.Stop()
	select {
	case chanWrite <- data:
	case <-t.C:
		return fmt.Errorf("time out in 3 seconds")
	}

	return nil
}

func (s *WsTcpSvr) Close(conn net.Conn) error {
	s.lockOfConnInfo.RLock()
	chanWrite := s.mapOfConnInfo[conn]

	s.lockOfConnInfo.RUnlock()

	if chanWrite == nil {
		return fmt.Errorf("connection doesn't exist")
	}

	t := time.NewTimer(3 * time.Second)
	defer t.Stop()
	select {
	case chanWrite <- nil:
	case <-t.C:
		return fmt.Errorf("time out in 3 seconds")
	}

	return nil
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

func (s *WsTcpSvr) runConnWrite(conn *websocket.Conn, chanWrite <-chan []byte) {
	defer conn.Close()

	for {
		writeData, ok := <-chanWrite
		if !ok { // chan is closed
			logger.Debugf("chanWrite is closed {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		if writeData == nil { // nil means close
			logger.Infof("A 'nil' is passed to chanWrite to close conn {local:%v, remote:%v}", conn.LocalAddr(), conn.RemoteAddr())
			break
		}

		conn.SetWriteDeadline(datetime.NowT().Add(s.wsWriteTimeout))
		err := conn.WriteMessage(websocket.BinaryMessage, writeData)
		if err != nil {
			logger.Errorf("Failed to write tcp data {err:%v, dataLen: %v}", err, len(writeData))
			break
		}
	}
}
