/**
 * Created by GoLand.
 * User: Iori
 * Date: 2021-10-12
 * Time: 16:24
 */

package ws_server

import (
	"GoOne/common/misc"
	"GoOne/lib/api/logger"
	"GoOne/lib/api/sharedstruct"
	"GoOne/lib/service/router"
	g1_protocol "GoOne/protobuf/protocol"
	"fmt"
	"runtime/debug"

	"github.com/gorilla/websocket"
)

const (
	// 用户连接超时时间
	heartbeatExpirationTime = 3 * 60
)

// 用户登录
type login struct {
	AppId  uint32
	UserId string
	Client *Client
}

// 读取客户端数据
func (l *login) GetKey() (key string) {
	key = GetUserKey(l.AppId, l.UserId)

	return
}

// 用户连接
type Client struct {
	Addr          string          // 客户端地址
	Socket        *websocket.Conn // 用户连接
	Send          chan []byte     // 待发送的数据
	AppId         uint32          // 登录的平台Id app/web/ios
	UserId        string          // 用户Id，用户登录以后才有
	FirstTime     int64           // 首次连接事件
	HeartbeatTime int64           // 用户上次心跳时间
	LoginTime     int64           // 登录时间 登录以后才有
}

// 初始化
func NewClient(appid uint32, addr string, socket *websocket.Conn, firstTime int64) (client *Client) {
	client = &Client{
		AppId:         appid,
		Addr:          addr,
		Socket:        socket,
		Send:          make(chan []byte, 100),
		FirstTime:     firstTime,
		HeartbeatTime: firstTime,
	}

	return
}

// 读取客户端数据
func (c *Client) GetKey() (key string) {
	key = GetUserKey(c.AppId, c.UserId)

	return
}

func (c *Client) OnPacket(addr string, data []byte) {
	headerLen := sharedstruct.ByteLenOfCSPacketHeader()
	logger.Debugf("on packet: {dataLen: %v, headerLen: %v, remoteAddr: %v}\n",
		len(data), headerLen, addr)

	packetHeader := sharedstruct.CSPacketHeader{}
	packetHeader.From(data)
	packetBody := data[headerLen:]
	logger.Debugf("[uid: %d] Received client packet: %#v", packetHeader.Uid, packetHeader)

	uid := packetHeader.Uid
	if uid > 0 {

	} else {
		if packetHeader.Cmd == uint32(g1_protocol.CMD_MAIN_LOGIN_REQ) {
		}
	}

	if misc.IsInnerCmd(packetHeader.Cmd) {
		logger.Debugf("Received an inner command from client: %#v", packetHeader)
		return
	}

	serverType := misc.ServerTypeInCmd(packetHeader.Cmd)
	router.SendMsgBySvrTypeConn(serverType, uid, packetHeader.Cmd, 0, 0, packetBody, addr)
}

// 读取客户端数据
func (c *Client) read() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("write stop", string(debug.Stack()), r)
		}
	}()

	defer func() {
		close(c.Send)
	}()

	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			logger.Errorf("read client[%v],msg :%s ", c.Addr, err)
			return
		}

		c.ProcessData(message)
		//c.OnPacket(c.Addr, message)
	}
}

// 向客户端写数据
func (c *Client) write() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("write stop", string(debug.Stack()), r)

		}
	}()

	defer func() {
		clientManager.Unregister <- c
		c.Socket.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				//fmt.Println("Client发送数据 关闭连接", c.Addr, "ok", ok)
				return
			}

			c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func (c *Client) SendMsg(msg []byte) {

	if c == nil {

		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("SendMsg stop:", r, string(debug.Stack()))
		}
	}()

	c.Send <- msg
}

// 发送pb数据
func (c *Client) SendPbMsg(data1 []byte, data2 []byte) {
	if c == nil {

		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("SendMsg stop:", r, string(debug.Stack()))
		}
	}()

	data := make([]byte, len(data1)+len(data2))
	pos := 0
	copy(data[pos:], data1)
	pos += len(data1)
	copy(data[pos:], data2)
	pos += len(data2)
	c.Send <- data
}

// 读取客户端数据
func (c *Client) close() {
	close(c.Send)
}

// 用户登录
func (c *Client) Login(appId uint32, userId string, loginTime int64) {
	c.AppId = appId
	c.UserId = userId
	c.LoginTime = loginTime
	// 登录成功=心跳一次
	c.Heartbeat(loginTime)
}

// 用户心跳
func (c *Client) Heartbeat(currentTime int64) {
	c.HeartbeatTime = currentTime
	return
}

// 心跳超时
func (c *Client) IsHeartbeatTimeout(currentTime int64) (timeout bool) {
	if c.HeartbeatTime+heartbeatExpirationTime*1000 <= currentTime {
		timeout = true
	}

	return
}

// 是否登录了
func (c *Client) IsLogin() (isLogin bool) {

	// 用户登录了
	if c.UserId != "" {
		isLogin = true

		return
	}

	return
}
