package errors

import "errors"

var (
	ErrNotLeader         = errors.New("not leader")            // 当前节点不是 Leader
	ErrClientNotExist    = errors.New("client does not exist") // 指定的客户端不存在
	UnSupportedOperation = errors.New("unsupported operation") // 不支持的操作
	UnSupportedMode      = errors.New("unsupported mode")      // 不支持的模式
	ErrCannotLoadConfig  = errors.New("cannot load config")    // 无法加载配置
	ErrCannotLoadState   = errors.New("cannot load state")     // 无法加载持久化状态
)
