package goWs

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"testing"
)

func TestConn(t *testing.T) {
	http.HandleFunc("/ws", wsHandler)
	_ = http.ListenAndServe("0.0.0.0:7777", nil)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn := NewWsConnection(empty{}, beat{})
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

var m sync.RWMutex

var col map[string]*websocket.Conn

func init() {
	col = make(map[string]*websocket.Conn)
}

type empty struct{}

func (e empty) Set(id string, wsConn *websocket.Conn) error {
	m.RLock()
	defer m.RUnlock()
	col[id] = wsConn
	return nil
}

func (e empty) Get(id string) (wsConn *websocket.Conn, err error) {
	m.RLock()
	defer m.RUnlock()
	conn, ok := col[id]
	if !ok {
		err = errors.New("id is not exist")
		return
	}
	return conn, nil
}

func (e empty) GetGroup(groupName string) (wsConnList []*websocket.Conn, err error) {
	m.RLock()
	defer m.RUnlock()
	for _, conn := range col {
		wsConnList = append(wsConnList, conn)
	}
	return wsConnList, nil
}

func (e empty) GetAll() (wsConnList []*websocket.Conn, err error) {
	m.RLock()
	defer m.RUnlock()
	for _, conn := range col {
		wsConnList = append(wsConnList, conn)
	}
	return wsConnList, nil
}

func (e empty) Del(id string) error {
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
