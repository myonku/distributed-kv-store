package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft/raft_store"
	"distributed-kv-store/internal/storage"
	"time"
)

// 上层写请求的统一入口（只在 Leader 上成功）
func (n *Node) Propose(ctx context.Context, cmd storage.Command) (ApplyResult, error) {
	// 只能在当前 Leader 上处理写请求
	if !n.IsLeader() {
		return ApplyResult{}, errors.ErrNotLeader
	}
	if n.logStore == nil {
		return ApplyResult{}, errors.ErrNotLeader
	}

	// 读取当前任期，确认自己仍是 Leader
	n.mu.Lock()
	if n.role != Leader {
		n.mu.Unlock()
		return ApplyResult{}, errors.ErrNotLeader
	}
	term := n.term
	n.mu.Unlock()

	// 为新日志计算索引，并追加到本地 Raft 日志
	lastIndex, err := n.logStore.LastIndex()
	if err != nil {
		return ApplyResult{}, err
	}
	newIndex := lastIndex + 1
	entry := raft_store.LogEntry{
		Index: newIndex,
		Term:  term,
		Cmd:   cmd,
	}
	if err := n.logStore.Append([]raft_store.LogEntry{entry}); err != nil {
		return ApplyResult{}, err
	}

	// 尽快触发一轮日志复制（AppendEntries），加速写入落到多数派。简化实现，直接广播一次
	go n.broadcastHeartbeat()

	// 等待该日志被 commit 并应用到状态机
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	// 等待日志应用完成或上下文取消
	for {
		select {
		case <-ctx.Done():
			return ApplyResult{Index: newIndex, Term: term, Err: ctx.Err()}, ctx.Err()
		case <-n.ctx.Done():
			return ApplyResult{Index: newIndex, Term: term, Err: context.Canceled}, context.Canceled
		case <-ticker.C:
			n.mu.Lock()
			role := n.role
			currentTerm := n.term
			applied := n.lastApplied
			n.mu.Unlock()

			// 如果在等待过程中失去 Leader 身份或任期变化，则认为失败
			if role != Leader || currentTerm != term {
				return ApplyResult{}, errors.ErrNotLeader
			}

			// 日志已经被 applyLoop 应用到状态机
			if applied >= newIndex {
				return ApplyResult{Index: newIndex, Term: term, Err: nil}, nil
			}
		}
	}
}

// 用于上层或控制面在 Leader 上发起配置变更
func (n *Node) ProposeConfChange(ctx context.Context, cc configs.ClusterConfigChange) (ApplyResult, error) {

	// 只能在当前 Leader 上发起配置变更
	if !n.IsLeader() {
		return ApplyResult{}, errors.ErrNotLeader
	}
	if n.logStore == nil {
		return ApplyResult{}, errors.ErrResourceNotInit
	}

	// 读取当前任期，确认自己仍是 Leader
	n.mu.Lock()
	if n.role != Leader {
		n.mu.Unlock()
		return ApplyResult{}, errors.ErrNotLeader
	}
	term := n.term
	n.mu.Unlock()

	// 为新日志计算索引，并追加到本地 Raft 日志
	lastIndex, err := n.logStore.LastIndex()
	if err != nil {
		return ApplyResult{}, err
	}
	newIndex := lastIndex + 1
	entry := raft_store.LogEntry{
		Index: newIndex,
		Term:  term,
		Type:  storage.EntryConfChange,
		Conf:  &cc,
	}
	if err := n.logStore.Append([]raft_store.LogEntry{entry}); err != nil {
		return ApplyResult{}, err
	}

	// 尽快触发一轮复制；当前实现仍是简化版，直接广播一次
	go n.broadcastHeartbeat()

	// 等待该配置变更日志被 commit 并应用（applyConfChange 执行完成）
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ApplyResult{Index: newIndex, Term: term, Err: ctx.Err()}, ctx.Err()
		case <-n.ctx.Done():
			return ApplyResult{Index: newIndex, Term: term, Err: context.Canceled}, context.Canceled
		case <-ticker.C:
			n.mu.Lock()
			role := n.role
			currentTerm := n.term
			applied := n.lastApplied
			n.mu.Unlock()

			// 如果在等待过程中失去 Leader 身份或任期变化，则认为失败
			if role != Leader || currentTerm != term {
				return ApplyResult{}, errors.ErrNotLeader
			}
			if applied >= newIndex {
				return ApplyResult{Index: newIndex, Term: term, Err: nil}, nil
			}
		}
	}
}

// 重置选举超时计时器（收到有效 RPC 时调用）
func (n *Node) resetElectionTimeout() {
	n.mu.Lock()
	n.electionResetAt = time.Now()
	n.mu.Unlock()
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
	return n.leaderID
}

// 加载持久化状态（term / votedFor / commitIndex）
func (n *Node) LoadState() error {
	hardState, err := n.hardStateStore.Load()
	if err != nil {
		return err
	}
	n.mu.Lock()
	n.term = hardState.Term
	n.votedFor = hardState.VotedFor
	n.commitIndex = hardState.CommitIndex
	n.mu.Unlock()
	return nil
}
