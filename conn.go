package gows

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"net"
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

// Message 定义了一个消息实体.
type Message struct {
	// MessageType The message types are defined in RFC 6455, section 11.8.
	MessageType int
	// Data 消息内容
	Data []byte
}

// Connection 维护的长连接.
type Connection struct {
	// id 标识id
	id string
	// conn 底层长连接
	conn *websocket.Conn
	// inChan 读队列
	inChan chan *Message
	// outChan 写队列
	outChan chan *Message
	// closeChan 关闭通知
	closeChan chan struct{}
	// heartbeatInterval 心跳检测间隔, 秒
	heartbeatInterval int
	// lastHeartbeatTime 最近一次心跳时间
	lastHeartbeatTime time.Time
	// mutex 保护 closeChan 只被执行一次
	mutex sync.Mutex
	// isClosed closeChan状态
	isClosed bool
}

// Options 可选参数
type Options struct {
	// InChanSize 读队列大小, 默认1024
	InChanSize int
	// OutChanSize 写队列大小, 默认1024
	OutChanSize int
	// HeartbeatInterval 心跳检测间隔, 当心跳间隔大于这个时间连接将断开, 默认300s
	HeartbeatInterval int
}

// NewConnection 新建 Connection实例.
func NewConnection(opts ...*Options) *Connection {
	inChanSize, outChanSize := DefaultInChanSize, DefaultOutChanSize
	heartbeatInterval := DefaultHeartbeatInterval
	if len(opts) > 0 {
		opt := opts[0]
		if opt.InChanSize > 0 {
			inChanSize = opt.InChanSize
		}
		if opt.OutChanSize > 0 {
			outChanSize = opt.OutChanSize
		}
		if opt.HeartbeatInterval > 0 {
			heartbeatInterval = opt.HeartbeatInterval
		}
	}
	return &Connection{
		id:                uuid.NewString(),
		conn:              nil,
		inChan:            make(chan *Message, inChanSize),
		outChan:           make(chan *Message, outChanSize),
		closeChan:         make(chan struct{}, 1),
		heartbeatInterval: heartbeatInterval,
		lastHeartbeatTime: time.Now(),
	}
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

// upgrade http升级websocket协议的配置. 允许所有CORS跨域请求.
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// readLoop 监听客户端消息
func (c *Connection) readLoop() {
	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			_ = c.close()
			goto EXIT
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
	timer := time.NewTimer(time.Duration(c.heartbeatInterval) * time.Second)
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
			timer.Reset(time.Duration(c.heartbeatInterval) * time.Second)
		case <-c.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// isAlive 判断连接是否活跃
func (c *Connection) isAlive() bool {
	return time.Since(c.lastHeartbeatTime) <= time.Duration(c.heartbeatInterval)*time.Second
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

// GetConnID 获取连接ID
func (c *Connection) GetConnID() string {
	return c.id
}

// GetRemoteAddr 获取远程地址
func (c *Connection) GetRemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// KeepHeartbeat 保持心跳
func (c *Connection) KeepHeartbeat() {
	c.lastHeartbeatTime = time.Now()
}
