package raft

import "time"

// 选举循环
func (n *Node) runElectionLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-time.After(n.electionTimeout):
			// 选举超时，尝试发起新一轮选举
			n.handleElectionTimeout()
		}
	}
}

// 心跳/复制循环（Leader）
func (n *Node) runHeartbeatLoop() {
	heartbeatTimer := time.NewTicker(n.heartbeatTimeout)
	defer heartbeatTimer.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-heartbeatTimer.C:
			// 仅在 Leader 身份时发送心跳/复制日志
			if !n.IsLeader() {
				continue
			}
			// TODO: 遍历 peers，构造 AppendEntriesRequest，通过 transport 发送心跳/增量日志
		}
	}
}

// 新增: 处理选举超时的内部逻辑，占位实现，后续可扩展为完整投票流程。
func (n *Node) handleElectionTimeout() {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 如果已经是 Leader，则无需重新选举
	if n.role == Leader {
		return
	}

	// 变为 Candidate，提升任期并给自己投票
	n.role = Candidate
	n.term++
	n.votedFor = n.id

	// TODO: 重置选举计票状态，并异步向 peers 发送 RequestVote RPC
}
