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
	conn := NewConnection(&callback{}, &beat{})
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
	con connect
)

func init() {
	con.connect = make(map[string]*Connection)
}

type connect struct {
	sync.RWMutex
	connect map[string]*Connection
}

func (c *connect) Add(ID string, wsConn *Connection) {
	c.RLock()
	defer c.RUnlock()
	c.connect[ID] = wsConn
}

func (c *connect) Get(ID string) (*Connection, error) {
	c.RLock()
	defer c.RUnlock()
	conn, ok := c.connect[ID]
	if !ok {
		return nil, errors.New("connect is not exist")
	}
	return conn, nil
}

func (c *connect) GetAll() []*Connection {
	c.RLock()
	defer c.RUnlock()
	connList := make([]*Connection, 0, len(c.connect))
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
