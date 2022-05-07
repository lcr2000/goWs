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

// WsConnection 维护的websocket长连接.
type WsConnection struct {
	// id
	id string
	// collect 在线集合接口
	collect Collection
	// heartBeater 维持心跳接口
	heartBeater HeartBeater
	// wsConn 底层websocket
	wsConn *websocket.Conn
	// inChan 读队列
	inChan chan *WsMessage
	// outChan 写队列
	outChan chan *WsMessage
	// closeChan 关闭通知
	closeChan chan struct{}
	// lastHeartbeatTime 最近一次心跳时间
	lastHeartbeatTime time.Time
	// mutex 保护closeChan只被执行一次
	mutex sync.Mutex
	// isClosed closeChan 状态
	isClosed bool
}

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

// upgrade http升级websocket协议的配置. 允许所有CORS跨域请求.
var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewWsConnection 新建一个WsConnection类型的长连接.
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

// GetWsConnId 获取WsConnection中唯一连接id
func (wc *WsConnection) GetWsConnId() string {
	return wc.id
}

func (wc *WsConnection) Close() error {
	return wc.removeAndClose()
}

// removeAndClose 移除维护的连接、关闭wsConn
func (wc *WsConnection) removeAndClose() error {
	_ = wc.wsConn.Close()
	wc.mutex.Lock()
	defer wc.mutex.Unlock()
	if !wc.isClosed {
		close(wc.closeChan)
		wc.isClosed = true
	}
	return wc.collect.Del(wc.id)
}

func (wc *WsConnection) Open(w http.ResponseWriter, r *http.Request) error {
	wsSocket, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	wc.setWsConn(wsSocket)
	err = wc.collect.Set(wc.id, wc)
	if err != nil {
		_ = wc.removeAndClose()
		return err
	}
	go wc.readLoop()
	go wc.writeLoop()
	return nil
}

// setWsConn 将升级得到的webSocket协议写入WsConnection中.
func (wc *WsConnection) setWsConn(wsConn *websocket.Conn) {
	wc.wsConn = wsConn
}

func (wc *WsConnection) Receive() (msg *WsMessage, err error) {
	select {
	case msg = <-wc.inChan:
	case <-wc.closeChan:
		err = ErrWsConnClose
	}
	return
}

func (wc *WsConnection) Write(msg *WsMessage) (err error) {
	select {
	case wc.outChan <- msg:
	case <-wc.closeChan:
		err = ErrWsConnClose
	}
	return
}

// readLoop 通过监听消息,向inChan写入WsMessage.
func (wc *WsConnection) readLoop() {
	for {
		msgType, data, err := wc.wsConn.ReadMessage()
		if err != nil {
			_ = wc.removeAndClose()
			goto EXIT
		}
		if wc.isPing(data) {
			continue
		}
		select {
		case wc.inChan <- &WsMessage{
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

// isHeatBeat 校验数据是否为客户端的Ping数据. 如果是,则向客户端写回Pong数据.
func (wc *WsConnection) isPing(msg []byte) bool {
	if !wc.heartBeater.IsPingMsg(msg) {
		return false
	}
	wc.keepAlive()
	_ = wc.Write(&WsMessage{
		To: MsgTo{
			ToType: ToConn,
			To:     wc.GetWsConnId(),
		},
		MessageType: websocket.TextMessage,
		Data:        wc.heartBeater.GetPongMsg(),
	})
	return true
}

// writeLoop 通过监听outChan队列,向websocket写入消息.
func (wc *WsConnection) writeLoop() {
	timer := time.NewTimer(time.Duration(wc.heartBeater.GetHeartbeatTime()) * time.Second)
	defer timer.Stop()
	for {
		select {
		case msg := <-wc.outChan:
			_ = wc.msgDispatch(msg)
		case <-timer.C:
			if !wc.isAlive() {
				_ = wc.removeAndClose()
				goto EXIT
			}
			timer.Reset(time.Duration(wc.heartBeater.GetHeartbeatTime()) * time.Second)
		case <-wc.closeChan:
			goto EXIT
		}
	}
EXIT:
	// 确保连接被关闭
	return
}

// msgDispatch为消息分发方法.
// 内部通过判断MsgTo消息中ToType类型，向pushAll、pushGroup、pushConn下发消息.
func (wc *WsConnection) msgDispatch(msg *WsMessage) error {
	switch msg.To.ToType {
	case ToAll:
		_ = wc.pushAll(msg)
	case ToGroup:
		_ = wc.pushGroup(msg)
	case ToConn:
		_ = wc.pushConn(msg)
	default:
	}
	return nil
}

// pushAll发送给所有在线连接.
func (wc *WsConnection) pushAll(msg *WsMessage) error {
	allConn, err := wc.collect.GetAll()
	if err != nil {
		return err
	}
	for _, conn := range allConn {
		err := conn.wsConn.WriteMessage(msg.MessageType, msg.Data)
		if err != nil {
			continue
		}
	}
	return nil
}

// pushGroup 把消息发送给群组在线连接.
func (wc *WsConnection) pushGroup(msg *WsMessage) error {
	groupConn, err := wc.collect.GetGroup(msg.To.To)
	if err != nil {
		return err
	}
	for _, conn := range groupConn {
		err := conn.wsConn.WriteMessage(msg.MessageType, msg.Data)
		if err != nil {
			continue
		}
	}
	return nil
}

// pushConn 把消息发送给指定连接.
func (wc *WsConnection) pushConn(msg *WsMessage) error {
	if msg.To.To == "" {
		return ErrIdEmpty
	}
	conn, err := wc.collect.Get(msg.To.To)
	if err != nil || conn == nil {
		return err
	}
	err = conn.wsConn.WriteMessage(msg.MessageType, msg.Data)
	if err != nil {
		return err
	}
	return nil
}

// keepAlive 更新WsConnection心跳
func (wc *WsConnection) keepAlive() {
	wc.lastHeartbeatTime = time.Now()
}

// isAlive 判断WsConnection是否太长时间没有心跳
func (wc *WsConnection) isAlive() bool {
	if time.Now().Sub(wc.lastHeartbeatTime) > time.Duration(wc.heartBeater.GetHeartbeatTime())*time.Second {
		return false
	}
	return true
}

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

// HeartBeater 业务方实现维持心跳的接口
type HeartBeater interface {
	// IsPingMsg 校验是否客户端Ping请求
	IsPingMsg(msg []byte) bool
	// GetPongMsg 获取服务端->客户端Pong请求数据
	GetPongMsg() []byte
	// GetHeartbeatTime 获取心跳间隔有效时间（单位是秒）。如果两次心跳大于这个有效时间，连接将断开
	GetHeartbeatTime() int
}
