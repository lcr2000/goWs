package goWs

import (
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

type Conn interface {
	Close() error
	Open(w http.ResponseWriter, r *http.Request) error
	Receive() (msg *WsMessage, err error)
	Write(msg *WsMessage) (err error)
}

// WsConnection表示维护的一个websocket类型.
type WsConnection struct {
	// id
	id string
	// 在线集合接口
	collect Collection
	// 维持心跳接口
	heartBeater HeartBeater
	// 底层websocket
	wsConn *websocket.Conn
	// 读队列
	inChan chan *WsMessage
	// 写队列
	outChan chan *WsMessage
	// 关闭通知
	closeChan chan struct{}
	// 最近一次心跳时间
	lastHeartbeatTime time.Time
	// 保护closeChan只被执行一次
	mutex sync.Mutex
	// closeChan 状态
	isClosed bool
}

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

// http升级websocket协议的配置. 允许所有CORS跨域请求.
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewWsConnection为新建一个WsConnection连接.
// collect为Collection接口类的具体实现；heartBeater为HeartBeater接口的具体实现.
func NewWsConnection(collect Collection, heartBeater HeartBeater) *WsConnection {
	return &WsConnection{
		id:                uuid.NewString(),
		collect:           collect,
		heartBeater:       heartBeater,
		wsConn:            nil,
		inChan:            make(chan *WsMessage, 1024),
		outChan:           make(chan *WsMessage, 1024),
		closeChan:         make(chan struct{}, 1),
		lastHeartbeatTime: time.Now(),
	}
}

// GetWsConnId获取WsConnection中唯一连接id
func (conn *WsConnection) GetWsConnId() string {
	return conn.id
}

func (conn *WsConnection) Close() error {
	return conn.removeAndClose()
}

// removeAndClose移除维护的连接、关闭wsConn
func (conn *WsConnection) removeAndClose() error {
	_ = conn.wsConn.Close()
	conn.mutex.Lock()
	defer conn.mutex.Unlock()
	if !conn.isClosed {
		close(conn.closeChan)
		conn.isClosed = true
	}
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
		_ = conn.removeAndClose()
		return err
	}
	go conn.readLoop()
	go conn.writeLoop()
	return nil
}

// setWsConn将升级得到的webSocket协议写入WsConnection中.
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

// readLoop通过监听消息,向inChan写入WsMessage.
func (conn *WsConnection) readLoop() {
	for {
		msgType, data, err := conn.wsConn.ReadMessage()
		if err != nil {
			_ = conn.removeAndClose()
			goto EXIT
		}
		if conn.isPing(data) {
			continue
		}
		select {
		case conn.inChan <- &WsMessage{
			MessageType: msgType,
			Data:        data,
		}:
		case <-conn.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// isHeatBeat校验数据是否为客户端的Ping数据. 如果是，则向客户端写回Pong数据.
func (conn *WsConnection) isPing(msg []byte) bool {
	if !conn.heartBeater.IsPingMsg(msg) {
		return false
	}
	conn.keepAlive()
	_ = conn.Write(&WsMessage{
		To: MsgTo{
			ToType: ToConn,
			To:     conn.GetWsConnId(),
		},
		MessageType: websocket.TextMessage,
		Data:        conn.heartBeater.GetPongMsg(),
	})
	return true
}

// writeLoop通过监听outChan队列,向websocket写入消息.
func (conn *WsConnection) writeLoop() {
	timer := time.NewTimer(time.Duration(conn.heartBeater.GetHeartbeatTime()) * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-conn.outChan:
			_ = conn.msgDispatch(msg)
		case <-timer.C:
			if !conn.isAlive() {
				_ = conn.removeAndClose()
				goto EXIT
			}
			timer.Reset(time.Duration(conn.heartBeater.GetHeartbeatTime()) * time.Second)
		case <-conn.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// msgDispatch为消息分发方法.
// 内部通过判断MsgTo消息中ToType类型，向pushAll、pushGroup、pushConn下发消息.
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

// pushAll发送给所有在线连接.
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

// pushGroup发送给群组在线连接.
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

// pushConn发送给指定连接.
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

// keepAlive更新WsConnection心跳
func (conn *WsConnection) keepAlive() {
	conn.lastHeartbeatTime = time.Now()
}

// isAlive判断WsConnection是否太长时间没有心跳
func (conn *WsConnection) isAlive() bool {
	if time.Now().Sub(conn.lastHeartbeatTime) > time.Duration(conn.heartBeater.GetHeartbeatTime())*time.Second {
		return false
	}
	return true
}

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

// HeartBeater描述了维持心跳所需要的数据
type HeartBeater interface {
	// 校验是否客户端Ping请求
	IsPingMsg(msg []byte) bool
	// 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// 获取心跳间隔有效时间（单位是秒）。如果两次心跳大于这个有效时间，连接将断开
	GetHeartbeatTime() int
}
