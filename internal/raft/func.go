package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft/raft_store"
	"distributed-kv-store/internal/storage"
	"maps"
	"time"
)

// 在 Leader 上执行一次线性一致读屏障
func (n *Node) LinearizableRead(ctx context.Context) error {
	if !n.IsLeader() {
		return errors.ErrNotLeader
	}
	if n.transport == nil || n.logStore == nil {
		return errors.ErrResourceNotInit
	}

	if err := n.ensureCommittedInCurrentTerm(ctx); err != nil {
		return err
	}
	if err := n.quorumHeartbeat(ctx); err != nil {
		return err
	}

	n.mu.RLock()
	commitIndex := n.commitIndex
	term := n.term
	n.mu.RUnlock()

	return n.waitAppliedTo(ctx, term, commitIndex)
}

// 上层写请求的统一入口（只在 Leader 上成功）
func (n *Node) Propose(ctx context.Context, cmd storage.Command) (ApplyResult, error) {
	if !n.IsLeader() {
		return ApplyResult{}, errors.ErrNotLeader
	}
	if n.transport == nil || n.logStore == nil {
		return ApplyResult{}, errors.ErrResourceNotInit
	}

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
	if err := n.waitAppliedTo(ctx, term, newIndex); err != nil {
		if err == errors.ErrNotLeader {
			return ApplyResult{}, err
		}
		return ApplyResult{Index: newIndex, Term: term, Err: err}, err
	}
	return ApplyResult{Index: newIndex, Term: term, Err: nil}, nil
}

// 用于上层或控制面在 Leader 上发起配置变更
func (n *Node) ProposeConfChange(ctx context.Context, cc configs.ClusterConfigChange) (ApplyResult, error) {

	if !n.IsLeader() {
		return ApplyResult{}, errors.ErrNotLeader
	}
	if n.transport == nil || n.logStore == nil {
		return ApplyResult{}, errors.ErrResourceNotInit
	}

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

	// 尽快触发一轮日志复制（AppendEntries），加速写入落到多数派。简化实现，直接广播一次
	go n.broadcastHeartbeat()

	// 等待该配置变更日志被 commit 并应用（applyConfChange 执行完成）
	if err := n.waitAppliedTo(ctx, term, newIndex); err != nil {
		if err == errors.ErrNotLeader {
			return ApplyResult{}, err
		}
		return ApplyResult{Index: newIndex, Term: term, Err: err}, err
	}
	return ApplyResult{Index: newIndex, Term: term, Err: nil}, nil
}

// 重置选举超时计时器（收到有效 RPC 时调用）
func (n *Node) resetElectionTimeout() {
	n.mu.Lock()
	n.electionResetAt = time.Now()
	n.mu.Unlock()
}

// 确保当前任期至少有一条已提交日志（必要时提交 noop）
func (n *Node) ensureCommittedInCurrentTerm(ctx context.Context) error {
	n.mu.RLock()
	if n.role != Leader {
		n.mu.RUnlock()
		return errors.ErrNotLeader
	}
	term := n.term
	commitIndex := n.commitIndex
	n.mu.RUnlock()

	// 若 commitIndex 对应条目属于当前任期，则可直接提供一致性读
	if commitIndex > 0 {
		if t, err := n.logStore.Term(commitIndex); err == nil && t == term {
			return nil
		}
	}

	// 否则先提交一条 noop，确保当前任期至少有一条已提交日志
	_, err := n.Propose(ctx, storage.Command{Op: storage.OpNoop})
	return err
}

// 在多数派节点上执行一次心跳
func (n *Node) quorumHeartbeat(ctx context.Context) error {
	n.mu.RLock()
	if n.role != Leader {
		n.mu.RUnlock()
		return errors.ErrNotLeader
	}
	term := n.term
	leaderID := n.id
	leaderCommit := n.commitIndex
	hbTimeout := n.heartbeatTimeout
	peersSnapshot := make(map[string]configs.ClusterNode, len(n.peers))
	maps.Copy(peersSnapshot, n.peers)
	n.mu.RUnlock()

	need := len(peersSnapshot)/2 + 1
	success := 1 // self
	ch := make(chan HeartbeatResult, len(peersSnapshot))

	req := &AppendEntriesRequest{
		Term:         term,
		LeaderID:     leaderID,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      nil,
		LeaderCommit: leaderCommit,
	}

	for id := range peersSnapshot {
		if id == n.id {
			continue
		}
		peerID := id
		go func() {
			rctx, cancel := context.WithTimeout(ctx, hbTimeout)
			defer cancel()

			resp, err := n.transport.SendAppendEntries(rctx, peerID, req)
			if err != nil || resp == nil {
				ch <- HeartbeatResult{Term: 0, Success: false}
				return
			}

			// 更高任期：立即退回 follower
			if resp.Term > term {
				n.mu.Lock()
				if resp.Term > n.term {
					n.term = resp.Term
					n.role = Follower
					n.votedFor = ""
					n.hardStateStore.Save(raft_store.HardState{
						Term:        n.term,
						VotedFor:    n.votedFor,
						CommitIndex: n.commitIndex,
					})
				}
				n.mu.Unlock()
				ch <- HeartbeatResult{Term: resp.Term, Success: false}
				return
			}

			ch <- HeartbeatResult{Term: resp.Term, Success: resp.Success && resp.Term == term}
		}()
	}

	replies := len(peersSnapshot) - 1
	for replies > 0 && success < need {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r := <-ch:
			replies--
			if r.Term > term {
				return errors.ErrNotLeader
			}
			if r.Success {
				success++
			}
		}
	}

	if success >= need {
		return nil
	}
	return errors.ErrQuorumNotReached
}

// 等待状态机应用到指定的日志索引，同时确保仍为当前任期的 Leader
func (n *Node) waitAppliedTo(ctx context.Context, term uint64, index uint64) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-n.ctx.Done():
			return context.Canceled
		case <-ticker.C:
			n.mu.RLock()
			role := n.role
			currentTerm := n.term
			applied := n.lastApplied
			n.mu.RUnlock()
			// 如果等待期间失去 Leader 身份，则返回错误
			if role != Leader || currentTerm != term {
				return errors.ErrNotLeader
			}
			if applied >= index {
				return nil
			}
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
