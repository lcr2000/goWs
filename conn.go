package goWs

import (
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

type Conn interface {
	Close() error
	Open(w http.ResponseWriter, r *http.Request) error
	Receive() (msg *WsMessage, err error)
	Write(msg *WsMessage) (err error)
}

// WsConnection表示维护的一个websocket类型
type WsConnection struct {
	// id
	id string
	// 在线集合接口
	collect Collection
	// websocket
	wsConn *websocket.Conn
	// 读队列
	inChan chan *WsMessage
	// 写队列
	outChan chan *WsMessage
	// 关闭通知
	closeChan chan struct{}
}

// 描述了一个消息实体
type WsMessage struct {
	// 消息发送对象
	To MsgTo
	// The message types are defined in RFC 6455, section 11.8.
	MessageType int
	// 消息内容
	Data []byte
}

// 定义了消息发送的对象类型
type ToType string

// 定义了消息发送的对象
type MsgTo struct {
	ToType ToType
	To     string
}

// http升级websocket协议的配置. 允许所有CORS跨域请求
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// 新建一个WsConnection连接. id为唯一连接id; collect为实现Collection接口类
func NewWsConnection(id string, collect Collection) *WsConnection {
	return &WsConnection{
		id:        id,
		collect:   collect,
		wsConn:    nil,
		inChan:    make(chan *WsMessage, 1024),
		outChan:   make(chan *WsMessage, 1024),
		closeChan: make(chan struct{}, 1),
	}
}

// 确保WsConnection连接中closeChan只会关闭一次
var once sync.Once

func (conn *WsConnection) Close() error {
	_ = conn.wsConn.Close()
	once.Do(func() {
		close(conn.closeChan)
	})
	return conn.collect.Del(conn.id)
}

func (conn *WsConnection) Open(w http.ResponseWriter, r *http.Request) error {
	wsSocket, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	conn.setWsConn(wsSocket)
	err = conn.collect.Set(conn.id, wsSocket)
	if err != nil {
		_ = conn.Close()
		return err
	}
	go conn.readLoop()
	go conn.writeLoop()
	return nil
}

// 将升级得到的WebSocket协议写入WsConnection中
func (conn *WsConnection) setWsConn(wsConn *websocket.Conn) {
	conn.wsConn = wsConn
}

func (conn *WsConnection) Receive() (msg *WsMessage, err error) {
	select {
	case msg = <-conn.inChan:
	case <-conn.closeChan:
		err = ErrWsConnClose
	}
	return
}

func (conn *WsConnection) Write(msg *WsMessage) (err error) {
	select {
	case conn.outChan <- msg:
	case <-conn.closeChan:
		err = ErrWsConnClose
	}
	return
}

func (conn *WsConnection) readLoop() {
	for {
		msgType, data, err := conn.wsConn.ReadMessage()
		if err != nil {
			_ = conn.Close()
			break
		}
		select {
		case conn.inChan <- &WsMessage{
			MessageType: msgType,
			Data:        data,
		}:
		case <-conn.closeChan:
			break
		}
	}
}

func (conn *WsConnection) writeLoop() {
	for {
		select {
		case msg := <-conn.outChan:
			_ = conn.msgDispatch(msg)
		case <-conn.closeChan:
			break
		}
	}
}

func (conn *WsConnection) msgDispatch(msg *WsMessage) error {
	switch msg.To.ToType {
	case ToAll:
		_ = conn.pushAll(msg)
	case ToGroup:
		_ = conn.pushGroup(msg)
	case ToConn:
		_ = conn.pushConn(msg)
	default:
	}
	return nil
}

func (conn *WsConnection) pushAll(msg *WsMessage) error {
	allWsConn, err := conn.collect.GetAll()
	if err != nil {
		return err
	}
	for _, wsConn := range allWsConn {
		err := wsConn.WriteMessage(msg.MessageType, msg.Data)
		if err != nil {
			continue
		}
	}
	return nil
}

func (conn *WsConnection) pushGroup(msg *WsMessage) error {
	allWsConn, err := conn.collect.GetGroup(msg.To.To)
	if err != nil {
		return err
	}
	for _, wsConn := range allWsConn {
		err := wsConn.WriteMessage(msg.MessageType, msg.Data)
		if err != nil {
			continue
		}
	}
	return nil
}

func (conn *WsConnection) pushConn(msg *WsMessage) error {
	if msg.To.To == "" {
		return ErrIdEmpty
	}
	wsConn, err := conn.collect.Get(msg.To.To)
	if err != nil || wsConn == nil {
		return err
	}
	err = wsConn.WriteMessage(msg.MessageType, msg.Data)
	if err != nil {
		return err
	}
	return nil
}

// 客户端维护的websocket在线连接接口
type Collection interface {
	Set(id string, wsConn *websocket.Conn) error
	Get(id string) (wsConn *websocket.Conn, err error)
	GetGroup(groupName string) (wsConnList []*websocket.Conn, err error)
	GetAll() (wsConnList []*websocket.Conn, err error)
	Del(id string) error
}
