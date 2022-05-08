# goWs
基于 gorilla 封装的并发安全的 Golang Websocket 组件

## 结构体

```go
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
// Heartbeat 业务方实现维持心跳的接口
type Heartbeat interface {
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
func NewConnection(heartBeater Heartbeat) *Connection
```

```go
func (c *Connection) GetConnID() string
func (c *Connection) Close() error
func (c *Connection) Open(w http.ResponseWriter, r *http.Request) error
func (c *Connection) Receive() (msg *WsMessage, err error)
func (c *Connection) Write(msg *WsMessage) (err error)
```

## 快速开始
 ```go
import (
	"fmt"
	ws "github.com/lcr2000/goWs"
	"net/http"
)

func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn := ws.NewConnection(&heartbeat{})
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
		err = conn.Write(&ws.Message{
			MessageType: msg.MessageType,
			Data:        msg.Data,
		})
		if err != nil {
			break
		}
	}
}

var (
	conManager *connectManager
)

func init() {
	conManager = &connectManager{
		connect: make(map[string]*ws.Connection),
	}
}

type connectManager struct {
	sync.RWMutex
	connect map[string]*ws.Connection
}

func (cm *connectManager) Add(ID string, conn *ws.Connection) {
	cm.RLock()
	defer cm.RUnlock()
	cm.connect[ID] = conn
}

func (cm *connectManager) Get(ID string) (*ws.Connection, error) {
	cm.RLock()
	defer cm.RUnlock()
	conn, ok := cm.connect[ID]
	if !ok {
		return nil, errors.New("connection is not exist")
	}
	return conn, nil
}

func (cm *connectManager) GetAll() []*ws.Connection {
	cm.RLock()
	defer cm.RUnlock()
	connList := make([]*ws.Connection, 0, len(cm.connect))
	for _, conn := range cm.connect {
		connList = append(connList, conn)
	}
	return connList
}

func (cm *connectManager) Del(ID string) {
	cm.RLock()
	defer cm.RUnlock()
	delete(cm.connect, ID)
}

type heartbeat struct{}

func (b *heartbeat) IsPingMsg(msg []byte) bool {
	return string(msg) == Ping
}

func (b *heartbeat) GetPongMsg() []byte {
	return []byte(Pong)
}

func (b *heartbeat) GetAliveTime() int {
	return 600
}
```