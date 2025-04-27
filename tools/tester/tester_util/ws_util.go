package tester_util

import (
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"sync"
	"time"
)

// WebSocketClient 结构体
type WebSocketClient struct {
	conn         *websocket.Conn
	OnConnect    func(*WebSocketClient)
	OnMessage    func(*WebSocketClient, []byte)
	OnClose      func(*WebSocketClient)
	Heartbeat    time.Duration
	stopSignal   chan struct{}
	mu           sync.Mutex
	retryCount   int
	MaxRetries   int
	url          string
	heartBeatMsg []byte

	Room *Room
}

// 创建 WebSocket 连接
func (c *WebSocketClient) Connect(urlStr string) error {
	c.url = urlStr
	err := c.connectInternal()
	if err != nil {
		log.Println("Initial connection failed:", err)
	}
	return err
}

func (c *WebSocketClient) connectInternal() error {
	u, err := url.Parse(c.url)
	if err != nil {
		return err
	}

	log.Println("Connecting to:", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.retryCount = 0 // 连接成功后重置重连计数
	c.mu.Unlock()

	if c.OnConnect != nil {
		c.OnConnect(c)
	}

	// 启动心跳检测
	go c.startHeartbeat()

	// 开启消息监听
	go c.listen()
	return nil
}

// 设置并立刻发送心跳内容
func (c *WebSocketClient) SetHeartbeat(msg []byte) {
	c.heartBeatMsg = msg
	err := c.SendMessage(c.heartBeatMsg)
	if err != nil {
		log.Println("Heartbeat failed:", err)
		c.handleReconnection()
		return
	}
}

// 启动心跳检测
func (c *WebSocketClient) startHeartbeat() {
	ticker := time.NewTicker(c.Heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.heartBeatMsg == nil {
				continue
			}
			err := c.SendMessage(c.heartBeatMsg)
			if err != nil {
				log.Println("Heartbeat failed:", err)
				c.handleReconnection()
				return
			}
		case <-c.stopSignal:
			return
		}
	}
}

// 监听 WebSocket 消息
func (c *WebSocketClient) listen() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Connection closed:", err)
			if c.OnClose != nil {
				c.OnClose(c)
			}
			c.handleReconnection()
			break
		}
		if c.OnMessage != nil {
			c.OnMessage(c, message)
		}
	}
}

// 发送消息
func (c *WebSocketClient) SendMessage(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, msg)
}

// 处理断线重连
func (c *WebSocketClient) handleReconnection() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.retryCount >= c.MaxRetries {
		log.Println("Max retries reached. Connection permanently closed.")
		return
	}

	log.Println("Attempting to reconnect...")
	time.Sleep(2 * time.Second) // 重连前等待 2 秒
	c.retryCount++
	err := c.connectInternal()
	if err != nil {
		log.Println("Reconnect failed, retrying...")
		go c.handleReconnection()
	}
}

// 关闭 WebSocket 连接
func (c *WebSocketClient) Close() {
	close(c.stopSignal)
	c.mu.Lock()
	c.conn.Close()
	c.mu.Unlock()

	if c.OnClose != nil {
		c.OnClose(c)
	}
}
