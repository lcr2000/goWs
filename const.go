package gows

import "errors"

var (
	// ErrConnClose 连接已关闭
	ErrConnClose = errors.New("connection already closed")
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a pong control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

const (
	// DefaultInChanSize 默认读队列大小
	DefaultInChanSize = 1024

	// DefaultOutChanSize 默认写队列大小
	DefaultOutChanSize = 1024

	// DefaultHeartbeatInterval 默认心跳检测间隔
	DefaultHeartbeatInterval = 300
)
