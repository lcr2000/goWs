# goWs
基于 gorilla 封装的并发安全的 Golang Websocket 组件

## 快速开始
 ```go
import (
	"fmt"
	ws "github.com/lcr2000/goWs"
	"net/http"
)

func WsHandler(w http.ResponseWriter, r *http.Request) {
    // 新建连接实例
	conn := ws.NewConnection()
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
		err = conn.Write(&ws.Message{
			MessageType: msg.MessageType,
			Data:        msg.Data,
		})
		if err != nil {
			break
		}
	}
}
```