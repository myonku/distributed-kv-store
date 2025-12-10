package store

import "context"

// 表示对底层状态机的一个逻辑操作。
// 无论是 Raft 日志 entry 还是一致性哈希节点上的本地写入，都可以统一抽象为 Command。
type Command struct {
	Op    string
	Key   string
	Value string
}

// 对底层 KV 状态机 + 持久化的抽象。
type Storage interface {
	AppendLog(ctx context.Context, cmd Command) (index uint64, err error) // 添加一条日志记录，返回该日志的索引
	ApplyLog(ctx context.Context, index uint64) error                     // 应用指定索引的日志到状态机
	Get(ctx context.Context, key string) (string, error)                  // 从状态机读取数据
	LastIndex() uint64                                                    // 返回当前最后一条日志的索引
}
