/**
 * Created by GoLand.
 * User: Iori
 * Date: 2021-10-12
 * Time: 16:24
 */

package ws_server

import (
	"GoOne/lib/api/datetime"
	"GoOne/lib/api/logger"
	"GoOne/lib/api/sharedstruct"
	g1_protocol "GoOne/protobuf/protocol"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cast"
)

// 连接管理
type ClientManager struct {
	Clients     map[*Client]bool   // 全部的连接
	ClientsLock sync.RWMutex       // 读写锁
	Users       map[string]*Client // 登录的用户 // appId+uuid
	UserLock    sync.RWMutex       // 读写锁
	Register    chan *Client       // 连接连接处理
	Login       chan *login        // 用户登录处理
	Unregister  chan *Client       // 断开连接处理程序
	Broadcast   chan []byte        // 广播 向全部成员发送数据
	HandlerFunc func(client *Client, message []byte)
}

func NewClientManager() (clientManager *ClientManager) {
	clientManager = &ClientManager{
		Clients:    make(map[*Client]bool),
		Users:      make(map[string]*Client),
		Register:   make(chan *Client, 1000),
		Login:      make(chan *login, 1000),
		Unregister: make(chan *Client, 1000),
		Broadcast:  make(chan []byte, 1000),
	}

	return
}

// 获取用户key
func GetUserKey(appId uint32, userId string) (key string) {
	key = fmt.Sprintf("%d_%s", appId, userId)

	return
}

/**************************  manager  ***************************************/

func (manager *ClientManager) InClient(client *Client) (ok bool) {
	manager.ClientsLock.RLock()
	defer manager.ClientsLock.RUnlock()

	// 连接存在，在添加
	_, ok = manager.Clients[client]

	return
}

// GetClients
func (manager *ClientManager) GetClients() (clients map[*Client]bool) {

	clients = make(map[*Client]bool)

	manager.ClientsRange(func(client *Client, value bool) (result bool) {
		clients[client] = value

		return true
	})

	return
}

// 遍历
func (manager *ClientManager) ClientsRange(f func(client *Client, value bool) (result bool)) {

	manager.ClientsLock.RLock()
	defer manager.ClientsLock.RUnlock()

	for key, value := range manager.Clients {
		result := f(key, value)
		if result == false {
			return
		}
	}

	return
}

// GetClientsLen
func (manager *ClientManager) GetClientsLen() (clientsLen int) {

	clientsLen = len(manager.Clients)

	return
}

// 添加客户端
func (manager *ClientManager) AddClients(client *Client) {
	manager.ClientsLock.Lock()
	defer manager.ClientsLock.Unlock()

	manager.Clients[client] = true
}

// 删除客户端
func (manager *ClientManager) DelClients(client *Client) {
	manager.ClientsLock.Lock()
	defer manager.ClientsLock.Unlock()

	if _, ok := manager.Clients[client]; ok {
		delete(manager.Clients, client)
	}
}

// 获取用户的连接
func (manager *ClientManager) GetUserClient(appId uint32, userId string) (client *Client) {

	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()

	userKey := GetUserKey(appId, userId)
	if value, ok := manager.Users[userKey]; ok {
		client = value
	}

	return
}

// GetClientsLen
func (manager *ClientManager) GetUsersLen() (userLen int) {
	userLen = len(manager.Users)

	return
}

// 添加用户
func (manager *ClientManager) AddUsers(key string, client *Client) {
	manager.UserLock.Lock()
	defer manager.UserLock.Unlock()

	manager.Users[key] = client
}

// 删除用户
func (manager *ClientManager) DelUsers(client *Client) (result bool) {
	manager.UserLock.Lock()
	defer manager.UserLock.Unlock()

	key := GetUserKey(client.AppId, client.UserId)
	if value, ok := manager.Users[key]; ok {
		// 判断是否为相同的用户
		if value.Addr != client.Addr {

			return
		}
		delete(manager.Users, key)
		result = true
	}

	return
}

// 获取用户的key
func (manager *ClientManager) GetUserKeys() (userKeys []string) {

	userKeys = make([]string, 0)
	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()
	for key := range manager.Users {
		userKeys = append(userKeys, key)
	}

	return
}

// 获取用户的key
func (manager *ClientManager) GetUserList(appId uint32) (userList []string) {

	userList = make([]string, 0)

	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()

	for _, v := range manager.Users {
		if v.AppId == appId {
			userList = append(userList, v.UserId)
		}
	}

	fmt.Println("GetUserList len:", len(manager.Users))
	return
}

// 获取用户的key
func (manager *ClientManager) GetUserClients() (clients []*Client) {

	clients = make([]*Client, 0)
	manager.UserLock.RLock()
	defer manager.UserLock.RUnlock()
	for _, v := range manager.Users {
		clients = append(clients, v)
	}

	return
}

// 向全部成员(除了自己)发送数据
func (manager *ClientManager) sendAll(message []byte, ignoreClient *Client) {

	clients := manager.GetUserClients()
	for _, conn := range clients {
		if conn != ignoreClient {
			conn.SendMsg(message)
		}
	}
}

// 向全部成员(除了自己)发送数据
func (manager *ClientManager) sendAppIdAll(message []byte, appId uint32, ignoreClient *Client) {

	clients := manager.GetUserClients()
	for _, conn := range clients {
		if conn != ignoreClient && conn.AppId == appId {
			conn.SendMsg(message)
		}
	}
}

// 用户建立连接事件
func (manager *ClientManager) EventRegister(client *Client) {
	manager.AddClients(client)

	//fmt.Println("EventRegister 用户建立连接", client.Addr)
	client.Send <- []byte("Connect  Success!")
}

// 用户登录
func (manager *ClientManager) EventLogin(login *login) {

	client := login.Client
	// 连接存在，在添加
	if manager.InClient(client) {
		userKey := login.GetKey()
		manager.AddUsers(userKey, login.Client)
	}

	fmt.Println("EventLogin 用户登录", client.Addr, login.AppId, login.UserId)

}

// 用户断开连接
func (manager *ClientManager) EventUnregister(client *Client) {
	manager.DelClients(client)

	// 删除用户连接
	deleteResult := manager.DelUsers(client)
	if deleteResult == false {
		// 不是当前连接的客户端

		return
	}

	// 清除redis登录数据
	//userOnline, err := cache.GetUserOnlineInfo(client.GetKey())
	//if err == nil {
	//	userOnline.LogOut()
	//	cache.SetUserOnlineInfo(client.GetKey(), userOnline)
	//}

	fmt.Println("EventUnregister 用户断开连接", client.Addr, client.AppId, client.UserId)
	//if client.UserId != "" {
	//	orderId := models.GetOrderIdTime()
	//	SendUserMessageAll(client.AppId, client.UserId, orderId, models.MessageCmdExit, "用户已经离开~")
	//}
}

// 管道处理程序
func (manager *ClientManager) start() {
	logger.Infof("ClientManager start loop!")

	for {
		select {
		case conn := <-manager.Register:
			manager.EventRegister(conn)

		case login := <-manager.Login:
			manager.EventLogin(login)

		case conn := <-manager.Unregister:
			manager.EventUnregister(conn)

		case message := <-manager.Broadcast:
			clients := manager.GetClients()
			for conn := range clients {
				select {
				case conn.Send <- message:
				default:
					close(conn.Send)
				}
			}

		}
	}
}

/**************************  manager info  ***************************************/
// 获取管理者信息
func GetManagerInfo(isDebug string) (managerInfo map[string]interface{}) {
	managerInfo = make(map[string]interface{})

	managerInfo["clientsLen"] = clientManager.GetClientsLen()        // 客户端连接数
	managerInfo["usersLen"] = clientManager.GetUsersLen()            // 登录用户数
	managerInfo["chanRegisterLen"] = len(clientManager.Register)     // 未处理连接事件数
	managerInfo["chanLoginLen"] = len(clientManager.Login)           // 未处理登录事件数
	managerInfo["chanUnregisterLen"] = len(clientManager.Unregister) // 未处理退出登录事件数
	managerInfo["chanBroadcastLen"] = len(clientManager.Broadcast)   // 未处理广播事件数

	if isDebug == "true" {
		addrList := make([]string, 0)
		clientManager.ClientsRange(func(client *Client, value bool) (result bool) {
			addrList = append(addrList, client.Addr)

			return true
		})

		users := clientManager.GetUserKeys()

		managerInfo["clients"] = addrList // 客户端列表
		managerInfo["users"] = users      // 登录用户列表
	}

	return
}

// 获取用户所在的连接
func GetUserClient(userId string) (client *Client) {
	client = clientManager.GetUserClient(DefaultAppId, userId)

	return
}

// kick
func (manager *ClientManager) KickById(uid uint64, reason g1_protocol.EKickOutReason) {
	client := GetUserClient(cast.ToString(uid))
	if client != nil {
		msg := g1_protocol.ScKickOut{Reason: reason}
		msgData, err := proto.Marshal(&msg)
		if err != nil {
			logger.Errorf("Marshal error in ScKickOut | %v", err)
			return
		}

		header := sharedstruct.CSPacketHeader{
			Uid:     uid,
			Cmd:     uint32(g1_protocol.CMD_SC_KICK_OUT),
			BodyLen: uint32(len(msgData)),
		}

		client.SendPbMsg(header.ToBytes(), msgData)
		return
	}
}

func (manager *ClientManager) SendByUid(uid uint64, data1 []byte, data2 []byte) error {
	client := GetUserClient(cast.ToString(uid))
	if client == nil {
		return fmt.Errorf("uid doesn't exist {uid: %v}", uid)
	}

	client.SendPbMsg(data1, data2)
	logger.Debugf("Send to client {uid: %v, len: %v}", uid, len(data1)+len(data2))
	return nil
}

func (manager *ClientManager) BroadcastByZone(zone int, header []byte, body []byte) {
	for _, client := range manager.Users {
		// TODO check zone
		client.SendPbMsg(header, body)
	}
}

// 定时清理超时连接
func ClearTimeoutConnections() {
	currentTime := datetime.NowMs()

	clients := clientManager.GetClients()
	for client := range clients {
		if client.IsHeartbeatTimeout(currentTime) {
			logger.Infof("HeartbeatTimeout : %v | %v | %s | %s", client.Addr, client.UserId, client.LoginTime,
				client.HeartbeatTime)

			client.Socket.Close()
		}
	}
}

// 获取全部用户
func GetUserList(appId uint32) (userList []string) {
	fmt.Println("获取全部用户", appId)

	userList = clientManager.GetUserList(appId)
	return
}

// 全员广播
func AllSendMessages(appId uint32, userId string, data string) {
	fmt.Println("全员广播", appId, userId, data)

	ignoreClient := clientManager.GetUserClient(appId, userId)
	clientManager.sendAppIdAll([]byte(data), appId, ignoreClient)
}
