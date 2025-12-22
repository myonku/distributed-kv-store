package storage

import (
	"context"
	"distributed-kv-store/configs"
)

type LogEntryType int
type CommandOpration string

const (
	EntryNormal     LogEntryType = iota // 普通日志条目
	EntryConfChange                     // 配置变更日志条目
)

const (
	OpPut    CommandOpration = "put"    // 设置键值对
	OpDelete CommandOpration = "delete" // 删除键值对
	OpNoop   CommandOpration = "noop"   // 空操作：用于 Raft barrier/一致性读
)

// 表示对底层状态机的一个逻辑操作。
// 无论是 Raft 日志 entry 还是一致性哈希节点上的本地写入，都可以统一抽象为 Command。
type Command struct {
	Op    CommandOpration
	Key   string
	Value string
}

// Raft 日志在底层存储中的原始结构
type RaftLogEntry struct {
	Index uint64
	Term  uint64
	Cmd   Command
	Type  LogEntryType                 // 日志类型
	Conf  *configs.ClusterConfigChange // 可选的集群配置变更
}

// Raft 硬状态在底层存储中的表示。
type RaftHardState struct {
	Term        uint64
	VotedFor    string
	CommitIndex uint64
}

// 对底层存储的抽象，同时服务于业务 KV（Standalone/状态机）、Raft 日志和 Raft 硬状态
type Storage interface {
	// 业务 KV 日志 + 状态机
	AppendLog(ctx context.Context, cmd Command) (index uint64, err error) // 添加一条业务日志记录，返回该日志的索引
	ApplyLog(ctx context.Context, index uint64) error                     // 将指定索引的业务日志应用到状态机
	Get(ctx context.Context, key string) (string, error)                  // 从状态机读取业务数据
	LastIndex() uint64                                                    // 当前最后一条业务日志的索引

	// Raft 日志相关接口
	AppendRaftLog(ctx context.Context, entries []RaftLogEntry) error             // 追加一批 Raft 日志
	RaftLogEntries(ctx context.Context, from, to uint64) ([]RaftLogEntry, error) // 读取 [from, to) 区间内的 Raft 日志
	RaftLogTerm(ctx context.Context, index uint64) (uint64, error)               // 获取指定索引的 Raft 日志任期
	RaftLogLastIndex(ctx context.Context) (uint64, error)                        // 当前 Raft 日志的最大索引
	RaftLogTruncateFrom(ctx context.Context, index uint64) error                 // 从 index 起（含）截断 Raft 日志

	// Raft 硬状态相关接口
	SaveRaftHardState(ctx context.Context, hs RaftHardState) error // 持久化保存当前 Raft 硬状态
	LoadRaftHardState(ctx context.Context) (RaftHardState, error)  // 读取上次保存的 Raft 硬状态

	Close() error // 关闭存储，释放资源
}
