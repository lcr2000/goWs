package ws

import "errors"

var (
	// websocket连接已关闭
	ErrWsConnClose = errors.New("websocket connection closed")
	// websocket唯一连接id不能为空
	ErrIdEmpty = errors.New("id is null")
)

// 描述了消息发送的对象
const (
	// 发送给所有在线连接
	ToAll ToType = "All"
	// 发送给群组在线连接
	ToGroup ToType = "Group"
	// 发送给指定连接
	ToConn ToType = "Conn"
)
