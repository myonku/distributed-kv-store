package errors

import err "errors"

var (
	ErrNotLeader           = err.New("not leader")                   // 当前节点不是 Leader
	ErrClientNotExist      = err.New("client does not exist")        // 指定的客户端不存在
	ErrUnSupportedMode     = err.New("unsupported mode")             // 不支持的模式
	ErrLogIndexMismatch    = err.New("log index mismatch")           // 日志索引不匹配
	ErrNoResourceRefrenced = err.New("no resource referenced")       // 未引用任何资源
	ErrResourceNotInit     = err.New("resource not initialized")     // 资源未初始化
	ErrInvalidConfChange   = err.New("invalid configuration change") // 无效的配置变更
	ErrUnkownEntryType     = err.New("unknown entry type")           // 未知的日志条目类型
	ErrQuorumNotReached    = err.New("quorum not reached")           // 未达到多数派
	ErrNoAvailablePeer     = err.New("no available peer")            // 没有可用的 peer
	ErrInvalidArgument     = err.New("invalid argument")             // 无效的参数
	ErrNoVNodeOwner        = err.New("no vnode owner")               // 没有虚拟节点的拥有者
)
