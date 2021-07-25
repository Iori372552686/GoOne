/**
 * Created by GoLand.
 * User: Iori
 * Date: 2020-07-25
 * Time: 16:24
 */

package web

import (
	"GoOne/common/misc"
	"fmt"
	"runtime/debug"

	`GoOne/lib/logger`
	`GoOne/lib/router`
	`GoOne/lib/sharedstruct`
	g1_protocol `GoOne/protobuf/protocol`

	`github.com/golang/protobuf/proto`
	"github.com/gorilla/websocket"
)

const (
	// 用户连接超时时间
	heartbeatExpirationTime = 3*60
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
	FirstTime     uint64          // 首次连接事件
	HeartbeatTime uint64          // 用户上次心跳时间
	LoginTime     uint64          // 登录时间 登录以后才有
}

// 初始化
func NewClient(addr string, socket *websocket.Conn, firstTime uint64) (client *Client) {
	client = &Client{
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


// 被Read协程调用，每个Connection对应一个Read协调
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

	}else {
		if packetHeader.Cmd == uint32(g1_protocol.CMD_MAIN_LOGIN_REQ) {
			req := &g1_protocol.LoginReq{}
			err := proto.Unmarshal(packetBody, req)
			if err != nil {
				logger.Errorf(" Fail to unmarshal LoginReq | %v ", err)
				return
			}

			if req.GetAccount() == "" || req.GetServerId() == "" {
				logger.Errorf(" LoginReq  -- > account or ServerId   error !!! ")
				return
			}

			//uid =  sid + aid
			//int_id,_:= strconv.Atoi(req.GetServerId() + strconv.Itoa(aid))
			//uid = uint64(int_id)

		}
	}

	if misc.IsInnerCmd(packetHeader.Cmd) {
		logger.Debugf("Received an inner command from client: %#v", packetHeader)
		return
	}


	serverType := misc.ServerTypeInCmd(packetHeader.Cmd)
	// router.SendMsgBySvrType(serverType, uid, packetHeader.Cmd, 0, 0, packetBody)
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
		fmt.Println("读取客户端数据 关闭send", c)
		close(c.Send)
	}()

	for {
		_, message, err := c.Socket.ReadMessage()
		if err != nil {
			fmt.Println("读取客户端数据 错误", c.Addr, err)

			return
		}

		// 处理程序
		fmt.Println("读取客户端数据 处理:", string(message))
		//ProcessData(c, message)
		c.OnPacket(c.Addr,message)
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
		fmt.Println("Client发送数据 defer", c)
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				// 发送数据错误 关闭连接
				fmt.Println("Client发送数据 关闭连接", c.Addr, "ok", ok)

				return
			}

			c.Socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}

//
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

	data := make([]byte, len(data1)+len(data2)); pos := 0
	copy(data[pos:], data1); pos += len(data1)
	copy(data[pos:], data2); pos += len(data2)
	c.Send <- data
}



// 读取客户端数据
func (c *Client) close() {
	close(c.Send)
}

// 用户登录
func (c *Client) Login(appId uint32, userId string, loginTime uint64) {
	c.AppId = appId
	c.UserId = userId
	c.LoginTime = loginTime
	// 登录成功=心跳一次
	c.Heartbeat(loginTime)
}

// 用户心跳
func (c *Client) Heartbeat(currentTime uint64) {
	c.HeartbeatTime = currentTime

	return
}

// 心跳超时
func (c *Client) IsHeartbeatTimeout(currentTime uint64) (timeout bool) {
	if c.HeartbeatTime+heartbeatExpirationTime <= currentTime {
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
