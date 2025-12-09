package raft

import (
	"context"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/store"
)

// 上层写请求的统一入口（只在 Leader 上成功）
func (n *Node) Propose(ctx context.Context, cmd store.Command) (ApplyResult, error) {
	if !n.IsLeader() {
		return ApplyResult{}, errors.ErrNotLeader
	}

	// 简化实现：
	// 1. 在本地日志尾部追加一条 entry；
	// 2. 直接将 commitIndex 推进到该 entry；
	// 3. 真实实现中，这一步应等待多数派复制成功再推进 commitIndex，并通过 applyCh 等待状态机应用完成。

	var entry LogEntry

	n.mu.Lock()
	newIndex := uint64(len(n.log)) + 1
	entry = LogEntry{
		Index: newIndex,
		Term:  n.term,
		Cmd:   cmd,
	}
	n.log = append(n.log, entry)
	if newIndex > n.commitIndex {
		n.commitIndex = newIndex
	}
	n.mu.Unlock()

	// 占位：此处应触发向 followers 发送 AppendEntries 复制日志
	// TODO: 调用内部复制逻辑，通过 transport 将 entry 发送给 peers

	// 简化：直接返回追加成功的结果，不等待真正的 apply 完成
	return ApplyResult{Index: entry.Index, Term: entry.Term, Err: nil}, nil
}

// 返回节点当前状态 snapshot
func (n *Node) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	return Status{
		ID:          n.id,
		Role:        n.role,
		Term:        n.term,
		CommitIndex: n.commitIndex,
		LastApplied: n.lastApplied,
	}
}

// 是否是 Leader
func (n *Node) IsLeader() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.role == Leader
}

// 返回当前 Leader ID（如果已知）
func (n *Node) LeaderID() string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.cfg.Raft.LeaderID
}
