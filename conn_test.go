package gows

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
	conn := NewConnection(&heartbeat{})
	if err := conn.Open(w, r); err != nil {
		return
	}
	conManager.Add(conn.GetConnID(), conn)
	for {
		// 读取消息
		msg, err := conn.Receive()
		if err != nil {
			break
		}
		fmt.Println(string(msg.Data))
		// 发送消息
		err = conn.Write(&Message{
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
		connect: make(map[string]*Connection),
	}
}

type connectManager struct {
	sync.RWMutex
	connect map[string]*Connection
}

func (cm *connectManager) Add(ID string, conn *Connection) {
	cm.RLock()
	defer cm.RUnlock()
	cm.connect[ID] = conn
}

func (cm *connectManager) Get(ID string) (*Connection, error) {
	cm.RLock()
	defer cm.RUnlock()
	conn, ok := cm.connect[ID]
	if !ok {
		return nil, errors.New("connection is not exist")
	}
	return conn, nil
}

func (cm *connectManager) GetAll() []*Connection {
	cm.RLock()
	defer cm.RUnlock()
	connList := make([]*Connection, 0, len(cm.connect))
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
