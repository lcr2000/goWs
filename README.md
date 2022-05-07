# goWs
基于gorilla封装的并发安全的Golang Websocket组件

## 结构体

WsConnection
```go
// WsConnection 维护的websocket长连接.
type WsConnection struct {
	// 隐藏了内部字段
}
```
WsMessage
```go
// WsMessage 定义了一个消息实体.
type WsMessage struct {
	// To 消息发送对象
	To MsgTo
	// MessageType The message types are defined in RFC 6455, section 11.8.
	MessageType int
	// Data 消息内容
	Data []byte
}

// ToType 定义了消息发送的对象类型.
type ToType string

// MsgTo 定义了消息发送的对象.
type MsgTo struct {
	ToType ToType
	To     string
}
```
通过`WsMessage`类，在内部进行对应消息的转发，如消息的一对一；一对多等。
1、`WsMessage`中`To`字段是内部自定义的一个类型`MsgTo`，指定了消息发送的对象，
`MsgTo`包含字段`ToType`以及字段`To`：字段`ToType`即消息发送的对象类型，分别是`ToAll`(发送给所有在线连接)、`ToGroup`(发送给群组在线连接)以及`ToConn`(发送给指定连接)；
字段`To`即具体对象。
2、`MessageType`是消息的具体类型；
3、`Data`是消息内容。

## 接口

```go
type Conn interface {
	Close() error
	Open(w http.ResponseWriter, r *http.Request) error
	Receive() (msg *WsMessage, err error)
	Write(msg *WsMessage) (err error)
}
```
接口`Conn`对外暴露了`4`个方法。方法`Close`即`Close`掉`NewWsConnection`构造的连接；方法`Open`会将`HTTP`服务器连接升级到`WebSocket`协议；
方法`Receive`接收客户端发送过来的消息；方法`Write`将消息发向客户端。

```go
// Collection 业务方实现维护长连接的接口.
type Collection interface {
	// Set 将连接写入集合
	Set(id string, wsConn *WsConnection) error
	// Get 从集合中获取对应id的连接
	Get(id string) (wsConn *WsConnection, err error)
	// GetGroup 通过groupId获取对应的连接列表
	GetGroup(groupId string) (wsConnList []*WsConnection, err error)
	// GetAll 获取所有连接
	GetAll() (wsConnList []*WsConnection, err error)
	// Del 将id对应的连接从集合中删除
	Del(id string) error
}
```
接口`Collection`业务方实现维护长连接的接口。使用前，请先实现此接口。

```go
// HeartBeater 业务方实现维持心跳的接口
type HeartBeater interface {
	// IsPingMsg 校验是否客户端Ping请求
	IsPingMsg(msg []byte) bool
	// GetPongMsg 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// GetHeartbeatTime 获取心跳间隔有效时间（单位是秒）。如果两次心跳大于这个有效时间，连接将断开
	GetHeartbeatTime() int
}
```
接口`HeartBeater`业务方实现维持心跳的接口。使用前，请先实现此接口。

## 方法说明

```go
func NewWsConnection(collect Collection, heartBeater HeartBeater) *WsConnection
```
方法`NewWsConnection`为新建一个`WsConnection`类型的长连接。
入参`collect`为`Collection`接口类的具体实现；入参`heartBeater`为`HeartBeater`接口的具体实现。

```go
func (conn *WsConnection) Close() error
func (conn *WsConnection) Open(w http.ResponseWriter, r *http.Request) error
func (conn *WsConnection) Receive() (msg *WsMessage, err error)
func (conn *WsConnection) Write(msg *WsMessage) (err error)
```
上面4个方法是接口`Conn`对外暴露的`4`个方法。

## 快速开始
 ```go
import (
	"fmt"
	ws "github.com/lcr2000/goWs"
	"net/http"
)

func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn := NewWsConnection(collect{}, beat{})
	if err := conn.Open(w, r); err != nil {
		return
	}
	for {
		// 读取消息
		msg, err := conn.Receive()
		if err != nil {
			break
		}
		fmt.Println(string(msg.Data))
		// 发送消息
		err = conn.Write(&WsMessage{
			To:          msg.To,
			MessageType: msg.MessageType,
			Data:        msg.Data,
		})
		if err != nil {
			break
		}
	}
}

type collect struct{}

func (c collect) Set(id string, wsConn *ws.WsConnection) error {
	m.RLock()
	defer m.RUnlock()
	col[id] = wsConn
	return nil
}

func (c collect) Get(id string) (wsConn *ws.WsConnection, err error) {
	m.RLock()
	defer m.RUnlock()
	conn, ok := col[id]
	if !ok {
		err = errors.New("id is not exist")
		return
	}
	return conn, nil
}

func (c collect) GetGroup(groupId string) (wsConnList []*ws.WsConnection, err error) {
	m.RLock()
	defer m.RUnlock()
	for _, conn := range col {
		wsConnList = append(wsConnList, conn)
	}
	return wsConnList, nil
}

func (c collect) GetAll() (wsConnList []*ws.WsConnection, err error) {
	m.RLock()
	defer m.RUnlock()
	for _, conn := range col {
		wsConnList = append(wsConnList, conn)
	}
	return wsConnList, nil
}

func (c collect) Del(id string) error {
	delete(col, id)
	return nil
}

type beat struct{}

func (b beat) IsPingMsg(msg []byte) bool {
	return false
}

func (b beat) GetPongMsg() []byte {
	return []byte{}
}

func (b beat) GetHeartbeatTime() int {
	return 10
}
```