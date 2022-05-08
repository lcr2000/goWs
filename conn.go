package gows

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

// Conn 长连接接口
type Conn interface {
	// Close 关闭连接
	Close() error
	// Open 开启连接
	Open(w http.ResponseWriter, r *http.Request) error
	// Receive 接收数据
	Receive() (msg *Message, err error)
	// Write 写入数据
	Write(msg *Message) (err error)
}

// Heartbeat 业务方实现维持心跳的接口
type Heartbeat interface {
	// IsPingMsg 校验是否Ping
	IsPingMsg(msg []byte) bool
	// GetPongMsg 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// GetAliveTime 获取连接活跃时间（秒）如果两次心跳大于这个有效时间连接将断开
	GetAliveTime() int
}

// Connection 维护的长连接.
type Connection struct {
	// id 标识id
	id string
	// heartbeat 心跳接口
	heartbeat Heartbeat
	// conn 底层长连接
	conn *websocket.Conn
	// inChan 读队列
	inChan chan *Message
	// outChan 写队列
	outChan chan *Message
	// closeChan 关闭通知
	closeChan chan struct{}
	// lastAliveTime 最近一次活跃时间
	lastAliveTime time.Time
	// mutex 保护closeChan只被执行一次
	mutex sync.Mutex
	// isClosed closeChan状态
	isClosed bool
}

// Message 定义了一个消息实体.
type Message struct {
	// MessageType The message types are defined in RFC 6455, section 11.8.
	MessageType int
	// Data 消息内容
	Data []byte
}

// upgrade http升级websocket协议的配置. 允许所有CORS跨域请求.
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewConnection 新建Connection实例.
func NewConnection(heartBeater Heartbeat) *Connection {
	return &Connection{
		id:            uuid.NewString(),
		heartbeat:     heartBeater,
		conn:          nil,
		inChan:        make(chan *Message, 1024),
		outChan:       make(chan *Message, 1024),
		closeChan:     make(chan struct{}, 1),
		lastAliveTime: time.Now(),
	}
}

// GetConnID 获取连接ID
func (c *Connection) GetConnID() string {
	return c.id
}

// Close 关闭连接
func (c *Connection) Close() error {
	return c.close()
}

// close 关闭连接
func (c *Connection) close() error {
	_ = c.conn.Close()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if !c.isClosed {
		close(c.closeChan)
		c.isClosed = true
	}
	return nil
}

// Open 开启连接
func (c *Connection) Open(w http.ResponseWriter, r *http.Request) error {
	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	go c.readLoop()
	go c.writeLoop()
	return nil
}

// Receive 接收数据
func (c *Connection) Receive() (msg *Message, err error) {
	select {
	case msg = <-c.inChan:
	case <-c.closeChan:
		err = ErrConnClose
	}
	return
}

// Write 写入数据
func (c *Connection) Write(msg *Message) (err error) {
	select {
	case c.outChan <- msg:
	case <-c.closeChan:
		err = ErrConnClose
	}
	return
}

// readLoop 监听客户端消息
func (c *Connection) readLoop() {
	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			_ = c.close()
			goto EXIT
		}
		c.keepAlive()
		if c.heartbeat.IsPingMsg(data) {
			_ = c.Write(&Message{
				MessageType: websocket.TextMessage,
				Data:        c.heartbeat.GetPongMsg(),
			})
			continue
		}
		select {
		case c.inChan <- &Message{
			MessageType: msgType,
			Data:        data,
		}:
		case <-c.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// writeLoop 向连接写入数据
func (c *Connection) writeLoop() {
	timer := time.NewTimer(time.Duration(c.heartbeat.GetAliveTime()) * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-c.outChan:
			_ = c.conn.WriteMessage(msg.MessageType, msg.Data)
		case <-timer.C:
			if !c.isAlive() {
				_ = c.close()
				goto EXIT
			}
			timer.Reset(time.Duration(c.heartbeat.GetAliveTime()) * time.Second)
		case <-c.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// keepAlive 保持活跃状态
func (c *Connection) keepAlive() {
	c.lastAliveTime = time.Now()
}

// isAlive 判断连接是否活跃
func (c *Connection) isAlive() bool {
	return time.Since(c.lastAliveTime) <= time.Duration(c.heartbeat.GetAliveTime())*time.Second
}
