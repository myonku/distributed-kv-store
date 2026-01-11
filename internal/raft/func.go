package raft

import (
	"context"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft/raft_store"
	"maps"
)

// 在 Leader 上执行一次线性一致读屏障
func (n *Node) LinearizableRead(ctx context.Context) error {

	if ctx == nil {
		ctx = context.Background()
	}

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
func (n *Node) Propose(ctx context.Context, cmd common.Command) (ApplyResult, error) {

	if ctx == nil {
		ctx = context.Background()
	}

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
func (n *Node) ProposeConfChange(ctx context.Context, cc common.ClusterConfigChange) (ApplyResult, error) {

	if ctx == nil {
		ctx = context.Background()
	}

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
		Type:  common.EntryConfChange,
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

// 确保当前任期至少有一条已提交日志（必要时提交 noop）
func (n *Node) ensureCommittedInCurrentTerm(ctx context.Context) error {

	if ctx == nil {
		ctx = context.Background()
	}

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
	_, err := n.Propose(ctx, common.Command{Op: common.OpNoop})
	return err
}

// 在多数派节点上执行一次心跳
func (n *Node) quorumHeartbeat(ctx context.Context) error {

	if ctx == nil {
		ctx = context.Background()
	}

	n.mu.RLock()
	if n.role != Leader {
		n.mu.RUnlock()
		return errors.ErrNotLeader
	}
	term := n.term
	leaderID := n.id
	leaderCommit := n.commitIndex
	hbTimeout := n.heartbeatTimeout
	peersSnapshot := make(map[string]RaftPeer, len(n.peers))
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
					n.leaderID = ""
					n.notifyStateChangeLocked()
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

	if ctx == nil {
		ctx = context.Background()
	}

	for {
		n.mu.Lock()
		if n.role != Leader || n.term != term {
			n.mu.Unlock()
			return errors.ErrNotLeader
		}
		if n.lastApplied >= index {
			n.mu.Unlock()
			return nil
		}
		stateCh := n.stateChangeCh
		waiter := make(chan ApplyResult, 1)
		if n.applyWaiters == nil {
			n.applyWaiters = make(map[uint64][]chan ApplyResult)
		}
		n.applyWaiters[index] = append(n.applyWaiters[index], waiter)
		n.mu.Unlock()

		select {
		case <-ctx.Done():
			n.unregisterApplyWaiter(index, waiter)
			return ctx.Err()
		case <-n.ctx.Done():
			n.unregisterApplyWaiter(index, waiter)
			return context.Canceled
		case <-stateCh:
			// role/term 可能已变化，清理 waiter 后重试或返回 ErrNotLeader
			n.unregisterApplyWaiter(index, waiter)
			continue
		case r := <-waiter:
			// 已应用到目标索引：仍需确认等待期间未失去 Leader 身份
			n.mu.RLock()
			role := n.role
			currentTerm := n.term
			n.mu.RUnlock()
			if role != Leader || currentTerm != term {
				return errors.ErrNotLeader
			}
			return r.Err
		}
	}
}

// 返回节点当前状态 snapshot
func (n *Node) Status() Status {
	n.mu.Lock()
	defer n.mu.Unlock()
	return Status{
		ID:            n.id,
		Role:          n.role,
		Term:          n.term,
		CommitIndex:   n.commitIndex,
		LastApplied:   n.lastApplied,
		CurrentLeader: n.leaderID,
	}
}

// 是否是 Leader
func (n *Node) IsLeader() bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.role == Leader
}

// 返回当前 Leader 信息（如果已知），否则返回空结构体
func (n *Node) LeaderInfo() RaftPeer {
	n.mu.Lock()
	defer n.mu.Unlock()
	// 从 peers 列表中查找当前 leaderID 对应的节点信息
	if leader, ok := n.peers[n.leaderID]; ok {
		return leader
	}
	return RaftPeer{}
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

// 返回节点运行状态
func (n *Node) IsRunning() bool {
	if n == nil {
		return false
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.running
}

// 通知状态变更（role/term）
func (n *Node) notifyStateChangeLocked() {
	if n.stateChangeCh != nil {
		close(n.stateChangeCh)
	}
	n.stateChangeCh = make(chan struct{})
}

// 注册等待应用某日志索引的通道
func (n *Node) unregisterApplyWaiter(index uint64, waiter chan ApplyResult) {
	n.mu.Lock()
	defer n.mu.Unlock()

	waiters := n.applyWaiters[index]
	for i, ch := range waiters {
		if ch == waiter {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(waiters) == 0 {
		delete(n.applyWaiters, index)
		return
	}
	n.applyWaiters[index] = waiters
}
