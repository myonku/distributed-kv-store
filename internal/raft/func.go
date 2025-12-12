package raft

import (
	"context"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft/raft_store"
	"distributed-kv-store/internal/storage"
	"time"
)

// 初始化时加载HardState和日志
func (n *Node) LoadState() error {
	hs, err := n.hardStateStore.Load()
	if err != nil {
		return err
	}
	n.term = hs.Term
	n.votedFor = hs.VotedFor
	n.commitIndex = hs.CommitIndex
	lastIndex, err := n.logStore.LastIndex()
	if err != nil {
		return err
	}
	n.lastApplied = lastIndex
	return nil
}

// 上层写请求的统一入口（只在 Leader 上成功）
func (n *Node) Propose(ctx context.Context, cmd storage.Command) (ApplyResult, error) {
	if !n.IsLeader() {
		return ApplyResult{}, errors.ErrNotLeader
	}

	var entry raft_store.LogEntry

	// 计算新的日志索引
	if n.logStore == nil {
		return ApplyResult{}, errors.ErrNotLeader
	}
	lastIndex, err := n.logStore.LastIndex()
	if err != nil {
		return ApplyResult{}, err
	}

	n.mu.Lock()
	term := n.term
	n.mu.Unlock()

	newIndex := lastIndex + 1
	entry = raft_store.LogEntry{
		Index: newIndex,
		Term:  term,
		Cmd:   cmd,
	}
	if err := n.logStore.Append([]raft_store.LogEntry{entry}); err != nil {
		return ApplyResult{}, err
	}

	// 触发一次心跳/日志复制
	if n.transport != nil {
		go n.broadcastHeartbeat()
	}

	// 等待该日志被多数节点复制
	if err := n.waitForCommit(ctx, newIndex); err != nil {
		return ApplyResult{}, err
	}

	// 可以在这里再增加一层对 applyCh 的等待逻辑，确保日志被应用到状态机后再返回。
	return ApplyResult{Index: entry.Index, Term: entry.Term, Err: nil}, nil
}

// 等待 commitIndex 推进到指定 index，表示该日志已在多数节点上复制成功。
func (n *Node) waitForCommit(ctx context.Context, index uint64) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		n.mu.Lock()
		committed := n.commitIndex >= index
		n.mu.Unlock()
		// 日志已提交，返回成功
		if committed {
			return nil
		}
		select {
		case <-ctx.Done(): // 调用方超时或取消
			return ctx.Err()
		case <-n.ctx.Done(): // 节点本身被关闭
			return context.Canceled
		case <-ticker.C: // 继续下一轮检查

		}
	}
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
