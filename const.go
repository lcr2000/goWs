package gows

import "errors"

var (
	// ErrConnClose 连接已关闭
	ErrConnClose = errors.New("connection already closed")
)

const (
	// Ping ping
	Ping = "ping"
	// Pong pong
	Pong = "pong"
)
