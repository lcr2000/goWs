package gows

import (
	"fmt"
	"net/http"
	"testing"
)

func TestConn(t *testing.T) {
	http.HandleFunc("/ws", wsHandler)
	_ = http.ListenAndServe("0.0.0.0:7777", nil)
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// 新建连接实例
	conn := NewConnection()
	// 开启连接
	if err := conn.Open(w, r); err != nil {
		return
	}
	// 关闭连接
	defer conn.Close()
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
