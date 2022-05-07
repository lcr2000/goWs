# goWs
基于gorilla封装的并发安全的Golang Websocket组件

## 结构体

```go
// Connection 维护的长连接
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
	inChan chan *WsMessage
	// outChan 写队列
	outChan chan *WsMessage
	// closeChan 关闭通知
	closeChan chan struct{}
	// lastAliveTime 最近一次活跃时间
	lastAliveTime time.Time
	// mutex 保护closeChan只被执行一次
	mutex sync.Mutex
	// isClosed closeChan状态
	isClosed bool
}
```

```go
// Message 定义了一个消息实体
type Message struct {
	// MessageType The message types are defined in RFC 6455, section 11.8.
	MessageType int
	// Data 消息内容
	Data []byte
}
```

## 接口

```go
// Conn 长连接接口
type Conn interface {
	// Close 关闭连接
	Close() error
	// Open 开启连接
	Open(w http.ResponseWriter, r *http.Request) error
	// Receive 接收数据
	Receive() (msg *WsMessage, err error)
	// Write 写入数据
	Write(msg *WsMessage) (err error)
}
```

```go
// Callback 业务方实现的回调接口
type Callback interface {
	// ConnClose 连接关闭
	ConnClose(id string)
}
```

```go
// HeartBeater 业务方实现维持心跳的接口
type HeartBeater interface {
	// IsPingMsg 校验是否Ping
	IsPingMsg(msg []byte) bool
	// GetPongMsg 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// GetAliveTime 获取连接活跃时间（秒）如果两次心跳大于这个有效时间连接将断开
	GetAliveTime() int
}
```

## 方法

```go
func NewConnection(callback Callback, heartBeater HeartBeater) *WsConnection
```

```go
func (conn *WsConnection) Close() error
func (conn *WsConnection) Open(w http.ResponseWriter, r *http.Request) error
func (conn *WsConnection) Receive() (msg *WsMessage, err error)
func (conn *WsConnection) Write(msg *WsMessage) (err error)
```

## 快速开始
 ```go
import (
	"fmt"
	ws "github.com/lcr2000/goWs"
	"net/http"
)

func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn := ws.NewConnection(&callback{}, &beat{})
	if err := conn.Open(w, r); err != nil {
		return
	}
	con.Add(conn.GetConnID(), conn)
	for {
		// 读取消息
		msg, err := conn.Receive()
		if err != nil {
			break
		}
		fmt.Println(string(msg.Data))
		// 发送消息
		err = conn.Write(&WsMessage{
			MessageType: msg.MessageType,
			Data:        msg.Data,
		})
		if err != nil {
			break
		}
	}
}

var (
	con connect
)

func init() {
	con.connect = make(map[string]*ws.Connection)
}

type connect struct {
	sync.RWMutex
	connect map[string]*Connection
}

func (c *connect) Add(ID string, wsConn *ws.Connection) {
	c.RLock()
	defer c.RUnlock()
	c.connect[ID] = wsConn
}

func (c *connect) Get(ID string) (*ws.Connection, error) {
	c.RLock()
	defer c.RUnlock()
	conn, ok := c.connect[ID]
	if !ok {
		return nil, errors.New("connect is not exist")
	}
	return conn, nil
}

func (c *connect) GetAll() []*ws.Connection {
	c.RLock()
	defer c.RUnlock()
	connList := make([]*ws.Connection, 0, len(c.connect))
	for _, conn := range c.connect {
		connList = append(connList, conn)
	}
	return connList
}

func (c *connect) Del(ID string) {
	delete(c.connect, ID)
}

type callback struct{}

func (c *callback) ConnClose(ID string) {
	con.Del(ID)
}

type beat struct{}

func (b *beat) IsPingMsg(msg []byte) bool {
	return string(msg) == Ping
}

func (b *beat) GetPongMsg() []byte {
	return []byte(Pong)
}

func (b *beat) GetAliveTime() int {
	return 600
}
```