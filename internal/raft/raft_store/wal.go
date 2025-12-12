package raft_store

import (
	"context"
	"distributed-kv-store/internal/storage"
)

// 面向 Raft Node 的日志存储接口
type RaftLogStore interface {
	Append(entries []LogEntry) error             // 追加日志条目
	Entries(from, to uint64) ([]LogEntry, error) // 获取指定范围的日志条目
	Term(index uint64) (uint64, error)           // 获取指定索引的日志任期
	Entry(index uint64) (LogEntry, error)        // 获取指定索引的日志条目
	LastIndex() (uint64, error)                  // 获取当前最大日志索引
	TruncateFrom(index uint64) error             // 从指定索引开始截断日志
}

// 日志条目
type LogEntry struct {
	Index uint64          // 日志索引
	Term  uint64          // 任期号
	Cmd   storage.Command // 存储命令
}

type raftLogStore struct {
	st storage.Storage
}

func NewRaftLogStore(st storage.Storage) *raftLogStore {
	return &raftLogStore{st: st}
}

func (r *raftLogStore) Append(entries []LogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// 将 raft_store.LogEntry 转为 storage.RaftLogEntry 交给底层存储
	raftEntries := make([]storage.RaftLogEntry, 0, len(entries))
	for _, e := range entries {
		raftEntries = append(raftEntries, storage.RaftLogEntry{
			Index: e.Index,
			Term:  e.Term,
			Cmd:   e.Cmd,
		})
	}
	return r.st.AppendRaftLog(context.TODO(), raftEntries)
}

func (r *raftLogStore) Entries(from, to uint64) ([]LogEntry, error) {
	raftEntries, err := r.st.RaftLogEntries(context.TODO(), from, to)
	if err != nil {
		return nil, err
	}
	res := make([]LogEntry, 0, len(raftEntries))
	for _, e := range raftEntries {
		res = append(res, LogEntry{
			Index: e.Index,
			Term:  e.Term,
			Cmd:   e.Cmd,
		})
	}
	return res, nil
}

func (r *raftLogStore) Entry(index uint64) (LogEntry, error) {
	entries, err := r.Entries(index, index+1)
	if err != nil {
		return LogEntry{}, err
	}
	if len(entries) == 0 {
		return LogEntry{}, nil
	}
	return entries[0], nil
}

func (r *raftLogStore) Term(index uint64) (uint64, error) {
	return r.st.RaftLogTerm(context.TODO(), index)
}

func (r *raftLogStore) LastIndex() (uint64, error) {
	return r.st.RaftLogLastIndex(context.TODO())
}

func (r *raftLogStore) TruncateFrom(index uint64) error {
	return r.st.RaftLogTruncateFrom(context.TODO(), index)
}

func (r *raftLogStore) Close() error {
	return nil
}
