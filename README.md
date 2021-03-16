# goWs
基于gorilla封装的并发安全的Golang Websocket组件

## 结构体

WsConnection
```go
// WsConnection表示维护的一个websocket类型.
type WsConnection struct {
	// 隐藏了内部字段
}
```
WsMessage
```go
// WsMessage描述了一个消息实体.
type WsMessage struct {
	// 消息发送对象
	To MsgTo
	// The message types are defined in RFC 6455, section 11.8.
	MessageType int
	// 消息内容
	Data []byte
}

// ToType定义了消息发送的对象类型.
type ToType string

// MsgTo定义了消息发送的对象.
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
// Collection描述了客户端维护的websocket在线连接接口的具体类型.
type Collection interface {
	// 将连接写入集合
	Set(id string, wsConn *websocket.Conn) error
	// 从集合中获取对应id的连接
	Get(id string) (wsConn *websocket.Conn, err error)
	// 通过groupName获取对应的连接列表
	GetGroup(groupName string) (wsConnList []*websocket.Conn, err error)
	// 获取所有连接
	GetAll() (wsConnList []*websocket.Conn, err error)
	// 将id对应的连接从集合中删除
	Del(id string) error
}
```
接口`Collection`描述了是如何维护一个在线连接。使用前，请先实现此接口。

```go
// HeartBeater描述了维持心跳所需要的数据
type HeartBeater interface {
	// 校验是否客户端Ping请求
	IsPingMsg(msg []byte) bool
	// 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// 获取心跳间隔有效时间（单位是秒）。如果两次心跳大于这个有效时间，连接将断开
	GetHeartbeatTime() int
}
```
接口`HeartBeater`描述了维持心跳所需要的数据。使用前，请先实现此接口。

## 方法说明

```go
func NewWsConnection(collect Collection, heartBeater HeartBeater) *WsConnection
```
方法`NewWsConnection`为新建一个`WsConnection`连接。
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
 	"errors"
 	"fmt"
 	"github.com/gorilla/websocket"
 	"net/http"
 )
 
func QuickStart(w http.ResponseWriter, r *http.Request) {
	conn := NewWsConnection(collect{}, beat{})
	if err := conn.Open(w, r); err != nil {
		return
	}
	for {
		msg, err := conn.Receive()
		if err != nil {
			break
		}
		fmt.Println(string(msg.Data))
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
 ```

collect为实现Collection接口的具体类。

 ```go
type collect struct{}

func (e collect) Set(id string, wsConn *websocket.Conn) error {
	return nil
}

func (e collect) Get(id string) (wsConn *websocket.Conn, err error) {
	return
}

func (e collect) GetGroup(groupName string) (wsConnList []*websocket.Conn, err error) {
	return
}

func (e collect) GetAll() (wsConnList []*websocket.Conn, err error) {
	return
}

func (e collect) Del(id string) error {
	return nil
}
```

beat为实现HeartBeater接口的具体类。

 ```go
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