package storage

import (
	"context"
	"distributed-kv-store/internal/common"
)

// 对底层存储的抽象
type Storage interface {
	// 业务 KV 日志 + 状态机

	AppendLog(ctx context.Context, cmd common.Command) (index uint64, err error)            // 添加一条业务日志记录，返回该日志的索引
	ApplyLog(ctx context.Context, index uint64) error                                       // 将指定索引的业务日志应用到状态机
	Get(ctx context.Context, key string) (string, error)                                    // 从状态机读取业务数据
	LastIndex() uint64                                                                      // 当前最后一条业务日志的索引
	BatchApply(ctx context.Context, cmds *[]common.Command) error                           // 批量添加业务日志并应用到状态机
	AppendBatchKV(ctx context.Context, kvs *[]common.KVPair) (startIndex uint64, err error) // 批量添加业务数据，返回起始日志索引
	GetBatch(ctx context.Context, startIndex, endIndex uint64) (*[]common.Command, error)   // 批量读取 [startIndex, endIndex) 区间内的业务日志
	GetHashRange(ctx context.Context, startHash, endHash uint32) (*[]common.KVPair, error)  // 按 Key 哈希范围读取业务数据（不经过日志）

	// Raft 日志相关接口

	AppendRaftLog(ctx context.Context, entries []common.RaftLogEntry) error             // 追加一批 Raft 日志
	RaftLogEntries(ctx context.Context, from, to uint64) ([]common.RaftLogEntry, error) // 读取 [from, to) 区间内的 Raft 日志
	RaftLogTerm(ctx context.Context, index uint64) (uint64, error)                      // 获取指定索引的 Raft 日志任期
	RaftLogLastIndex(ctx context.Context) (uint64, error)                               // 当前 Raft 日志的最大索引
	RaftLogTruncateFrom(ctx context.Context, index uint64) error                        // 从 index 起（含）截断 Raft 日志

	// Raft 硬状态相关接口

	SaveRaftHardState(ctx context.Context, hs common.RaftHardState) error // 持久化保存当前 Raft 硬状态
	LoadRaftHardState(ctx context.Context) (common.RaftHardState, error)  // 读取上次保存的 Raft 硬状态

	// 为Gossip + Chash 模式下的迁移记录提供存储支持

	GetMoveRangeRecord(ctx context.Context, moveID uint32) (exists bool, err error) // 获取指定迁移 ID 的迁移记录是否存在
	SaveMoveRangeRecord(ctx context.Context, moveID uint32) error                   // 保存指定迁移 ID 的迁移记录

	Close() error // 关闭存储，释放资源
}
