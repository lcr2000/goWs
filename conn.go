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

// Connection 维护的长连接.
type Connection struct {
	// id 标识id
	id string
	// callback 回调接口
	callback Callback
	// heartBeater 心跳接口
	heartBeater HeartBeater
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
func NewConnection(callback Callback, heartBeater HeartBeater) *Connection {
	return &Connection{
		id:            uuid.NewString(),
		callback:      callback,
		heartBeater:   heartBeater,
		conn:          nil,
		inChan:        make(chan *Message, 1024),
		outChan:       make(chan *Message, 1024),
		closeChan:     make(chan struct{}, 1),
		lastAliveTime: time.Now(),
	}
}

// GetConnID 获取连接ID
func (wc *Connection) GetConnID() string {
	return wc.id
}

// Close 关闭连接
func (wc *Connection) Close() error {
	return wc.removeAndClose()
}

// removeAndClose 移除维护的连接、关闭wsConn
func (wc *Connection) removeAndClose() error {
	_ = wc.conn.Close()
	wc.mutex.Lock()
	defer wc.mutex.Unlock()
	if !wc.isClosed {
		close(wc.closeChan)
		wc.isClosed = true
	}
	wc.callback.ConnClose(wc.id)
	return nil
}

// Open 开启连接
func (wc *Connection) Open(w http.ResponseWriter, r *http.Request) error {
	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	wc.conn = conn
	go wc.readLoop()
	go wc.writeLoop()
	return nil
}

// Receive 接收数据
func (wc *Connection) Receive() (msg *Message, err error) {
	select {
	case msg = <-wc.inChan:
	case <-wc.closeChan:
		err = ErrConnClose
	}
	return
}

// Write 写入数据
func (wc *Connection) Write(msg *Message) (err error) {
	select {
	case wc.outChan <- msg:
	case <-wc.closeChan:
		err = ErrConnClose
	}
	return
}

// readLoop 通过监听消息,向inChan写入数据
func (wc *Connection) readLoop() {
	for {
		msgType, data, err := wc.conn.ReadMessage()
		if err != nil {
			_ = wc.removeAndClose()
			goto EXIT
		}
		if wc.isPing(data) {
			continue
		}
		select {
		case wc.inChan <- &Message{
			MessageType: msgType,
			Data:        data,
		}:
		case <-wc.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// isPing 是否Ping
func (wc *Connection) isPing(msg []byte) bool {
	if !wc.heartBeater.IsPingMsg(msg) {
		return false
	}
	wc.keepAlive()
	_ = wc.Write(&Message{
		MessageType: websocket.TextMessage,
		Data:        wc.heartBeater.GetPongMsg(),
	})
	return true
}

// writeLoop 通过监听outChan队列,向连接写入消息
func (wc *Connection) writeLoop() {
	timer := time.NewTimer(time.Duration(wc.heartBeater.GetAliveTime()) * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-wc.outChan:
			wc.keepAlive()
			_ = wc.conn.WriteMessage(msg.MessageType, msg.Data)
		case <-timer.C:
			if !wc.isAlive() {
				_ = wc.removeAndClose()
				goto EXIT
			}
			timer.Reset(time.Duration(wc.heartBeater.GetAliveTime()) * time.Second)
		case <-wc.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// keepAlive 保持活跃状态
func (wc *Connection) keepAlive() {
	wc.lastAliveTime = time.Now()
}

// isAlive 判断连接是否活跃
func (wc *Connection) isAlive() bool {
	return time.Since(wc.lastAliveTime) <= time.Duration(wc.heartBeater.GetAliveTime())*time.Second
}

// Callback 业务方实现的回调接口
type Callback interface {
	// ConnClose 连接关闭
	ConnClose(id string)
}

// HeartBeater 业务方实现维持心跳的接口
type HeartBeater interface {
	// IsPingMsg 校验是否Ping
	IsPingMsg(msg []byte) bool
	// GetPongMsg 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// GetAliveTime 获取连接活跃时间（秒）如果两次心跳大于这个有效时间连接将断开
	GetAliveTime() int
}
