package errors

import err "errors"

var (
	ErrNotLeader        = err.New("not leader")            // 当前节点不是 Leader
	ErrClientNotExist   = err.New("client does not exist") // 指定的客户端不存在
	ErrUnSupportedMode  = err.New("unsupported mode")      // 不支持的模式
	ErrLogIndexMismatch = err.New("log index mismatch")    // 日志索引不匹配
)
