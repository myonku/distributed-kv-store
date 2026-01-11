package common

import (
	"distributed-kv-store/configs"
)

type LogEntryType int
type CommandOperation string
type ConfChangeType int // 配置变更类型

const (
	EntryNormal     LogEntryType = iota // 普通日志条目
	EntryConfChange                     // 配置变更日志条目
)

const (
	ConfChangeAddNode ConfChangeType = iota
	ConfChangeRemoveNode
)

// 集群配置变更条目
type ClusterConfigChange struct {
	Type ConfChangeType
	Node configs.ClusterNode
}

const (
	OpPut    CommandOperation = "put"    // 设置键值对
	OpDelete CommandOperation = "delete" // 删除键值对
	OpNoop   CommandOperation = "noop"   // 空操作：用于 Raft barrier/一致性读
)

// 表示一个键值对
type KVPair struct {
	Key   string
	Value string
}

// 表示对底层状态机的一个逻辑操作
type Command struct {
	Op    CommandOperation
	Key   string
	Value string
}

// Raft 日志在底层存储中的原始结构
type RaftLogEntry struct {
	Index uint64
	Term  uint64
	Cmd   Command
	Type  LogEntryType         // 日志类型
	Conf  *ClusterConfigChange // 可选的集群配置变更
}

// Raft 硬状态在底层存储中的表示
type RaftHardState struct {
	Term        uint64
	VotedFor    string
	CommitIndex uint64
}
