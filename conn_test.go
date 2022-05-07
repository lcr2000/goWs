package goWs

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func TestConn(t *testing.T) {
	http.HandleFunc("/ws", wsHandler)
	_ = http.ListenAndServe("0.0.0.0:7777", nil)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
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

var m sync.RWMutex

var col map[string]*WsConnection

func init() {
	col = make(map[string]*WsConnection)
}

type collect struct{}

func (c collect) Set(id string, wsConn *WsConnection) error {
	m.RLock()
	defer m.RUnlock()
	col[id] = wsConn
	return nil
}

func (c collect) Get(id string) (wsConn *WsConnection, err error) {
	m.RLock()
	defer m.RUnlock()
	conn, ok := col[id]
	if !ok {
		err = errors.New("id is not exist")
		return
	}
	return conn, nil
}

func (c collect) GetGroup(groupId string) (wsConnList []*WsConnection, err error) {
	m.RLock()
	defer m.RUnlock()
	for _, conn := range col {
		wsConnList = append(wsConnList, conn)
	}
	return wsConnList, nil
}

func (c collect) GetAll() (wsConnList []*WsConnection, err error) {
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
