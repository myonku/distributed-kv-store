package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/raft/raft_store"
	"maps"
	"math/rand"
	"time"
)

// 选举循环
func (n *Node) runElectionLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-time.After(n.electionTimeout + time.Duration(rand.Intn(100))*time.Millisecond): // 加入随机抖动
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
			if !n.IsLeader() {
				continue
			}
			n.broadcastHeartbeat()
		}
	}
}

// 处理选举超时的内部逻辑
func (n *Node) handleElectionTimeout() {
	n.mu.Lock()

	if n.role == Leader {
		n.mu.Unlock()
		return
	}

	// 提升任期，转为 Candidate，并给自己投票
	n.role = Candidate
	n.term++
	n.votedFor = n.id
	n.voteCount = 1 // 先算上自己的一票
	// 持久化当前任期和投票信息
	n.hardStore.Save(raft_store.HardState{
		Term:        n.term,
		VotedFor:    n.votedFor,
		CommitIndex: n.commitIndex,
	})

	// 记录当前任期和日志信息，用于构造 RequestVote
	currentTerm := n.term
	var lastIndex uint64
	var lastTerm uint64
	if n.logStore != nil {
		if li, err := n.logStore.LastIndex(); err == nil {
			lastIndex = li
			if lastIndex > 0 {
				if lt, err := n.logStore.Term(lastIndex); err == nil {
					lastTerm = lt
				}
			}
		}
	}

	// 拷贝 peers 列表供锁外使用
	peers := append([]configs.RaftPeer(nil), n.peers...)
	n.mu.Unlock()

	if n.transport == nil {
		return
	}

	for _, p := range peers {
		// 不给自己发 RequestVote
		if p.ID == n.id {
			continue
		}

		req := &RequestVoteRequest{
			Term:         currentTerm,
			CandidateID:  n.id,
			LastLogIndex: lastIndex,
			LastLogTerm:  lastTerm,
		}

		go func(peerID string, termAtStart uint64) {
			ctx, cancel := context.WithTimeout(n.ctx, n.electionTimeout)
			defer cancel()

			resp, err := n.transport.SendRequestVote(ctx, peerID, req)
			if err != nil || resp == nil {
				// 简化：忽略错误，等下一轮选举重试
				return
			}

			n.mu.Lock()
			defer n.mu.Unlock()

			// 如果发现更高的任期，立即退回 Follower
			if resp.Term > n.term {
				n.term = resp.Term
				n.role = Follower
				n.votedFor = ""
				n.hardStore.Save(raft_store.HardState{
					Term:        n.term,
					VotedFor:    n.votedFor,
					CommitIndex: n.commitIndex,
				})
				return
			}

			// 任期或角色已变化（比如已经选出别的 Leader），忽略旧选举结果
			if n.term != termAtStart || n.role != Candidate {
				return
			}

			if resp.VoteGranted {
				n.voteCount++
				// 简单多数判断：超过集群一半节点即成为 Leader
				if n.voteCount > len(n.peers)/2 {
					n.role = Leader
					// 初始化所有 follower 的 nextIndex / matchIndex
					var nextIdx uint64 = 1
					if n.logStore != nil {
						if li, err := n.logStore.LastIndex(); err == nil {
							nextIdx = li + 1
						}
					}
					for _, peer := range n.peers {
						if peer.ID == n.id {
							continue
						}
						n.nextIndex[peer.ID] = nextIdx
						n.matchIndex[peer.ID] = 0
					}

					// 尽快发出一轮心跳／增量复制，让其他节点感知新 Leader
					go n.broadcastHeartbeat()
				}
			}
		}(p.ID, currentTerm)
	}
}

// 向所有 peer 发送一次 AppendEntries（心跳或增量日志）。
func (n *Node) broadcastHeartbeat() {
	if n.transport == nil {
		return
	}

	// 读取当前 leader 的任期、日志、nextIndex 等状态
	n.mu.Lock()
	if n.role != Leader {
		n.mu.Unlock()
		return
	}

	term := n.term
	leaderID := n.id
	leaderCommit := n.commitIndex
	nextIdxSnapshot := make(map[string]uint64, len(n.nextIndex))
	maps.Copy(nextIdxSnapshot, n.nextIndex)

	peers := append([]configs.RaftPeer(nil), n.peers...)
	n.mu.Unlock()

	var lastIndex uint64
	if n.logStore != nil {
		if li, err := n.logStore.LastIndex(); err == nil {
			lastIndex = li
		}
	}

	for _, p := range peers {
		if p.ID == leaderID {
			continue
		}

		// 计算该 follower 需要从哪个索引开始发送增量日志
		ni := nextIdxSnapshot[p.ID]
		if ni == 0 {
			// 如果未初始化，默认从日志尾后一个位置开始（只发心跳）
			ni = lastIndex + 1
		}

		prevLogIndex := ni - 1
		var prevLogTerm uint64
		if prevLogIndex > 0 && n.logStore != nil {
			if t, err := n.logStore.Term(prevLogIndex); err == nil {
				prevLogTerm = t
			}
		}

		// 发送从 nextIndex 开始的增量日志；如果没有新日志，则 Entries 为空，相当于心跳
		var entries []raft_store.LogEntry
		if n.logStore != nil && ni <= lastIndex {
			if es, err := n.logStore.Entries(ni, lastIndex+1); err == nil {
				entries = es
			}
		}

		req := &AppendEntriesRequest{
			Term:         term,
			LeaderID:     leaderID,
			PrevLogIndex: prevLogIndex,
			PrevLogTerm:  prevLogTerm,
			Entries:      entries,
			LeaderCommit: leaderCommit,
		}

		// 异步发送 AppendEntries，并根据返回结果简单更新 nextIndex/matchIndex
		go func(peerID string, req *AppendEntriesRequest, termAtSend uint64) {
			ctx, cancel := context.WithTimeout(n.ctx, n.heartbeatTimeout)
			defer cancel()
			resp, err := n.transport.SendAppendEntries(ctx, peerID, req)
			if err != nil || resp == nil {
				// 简化：忽略错误，下一轮心跳重试
				return
			}

			n.mu.Lock()
			defer n.mu.Unlock()

			// 如果发现更高任期，退回 Follower
			if resp.Term > n.term {
				n.term = resp.Term
				n.role = Follower
				n.votedFor = ""
				n.hardStore.Save(raft_store.HardState{
					Term:        n.term,
					VotedFor:    n.votedFor,
					CommitIndex: n.commitIndex,
				})
				return
			}

			// 任期或角色已变化，忽略旧的响应
			if n.role != Leader || n.term != termAtSend {
				return
			}

			if resp.Success {
				// follower 成功复制日志，更新 matchIndex 和 nextIndex
				match := req.PrevLogIndex + uint64(len(req.Entries))
				n.matchIndex[peerID] = match
				n.nextIndex[peerID] = match + 1

				// 根据各节点 matchIndex 寻找大多数节点都已复制的最大索引 N，推进 commitIndex
				var lastIndex uint64
				if n.logStore != nil {
					if li, err := n.logStore.LastIndex(); err == nil {
						lastIndex = li
					}
				}
				N := n.commitIndex
				for i := n.commitIndex + 1; i <= lastIndex; i++ {
					count := 1
					for _, peer := range n.peers {
						if peer.ID == n.id { // 忽略自己
							continue
						}
						if n.matchIndex[peer.ID] >= i { // 该节点已复制到 i
							count++
						}
					}
					// 大多数节点已复制到 i，且该日志条目属于当前任期
					if count > len(n.peers)/2 && n.logStore != nil {
						if t, err := n.logStore.Term(i); err == nil && t == n.term {
							N = i
						}
					}
				}
				if N > n.commitIndex {
					n.commitIndex = N
					// 持久化 commitIndex
					n.hardStore.Save(raft_store.HardState{
						Term:        n.term,
						VotedFor:    n.votedFor,
						CommitIndex: n.commitIndex,
					})
				}
			} else {
				// 复制失败（例如 prevLog 不匹配），查找性地回退 nextIndex
				for n.nextIndex[peerID] > 1 {
					n.nextIndex[peerID]--
					prevIdx := n.nextIndex[peerID] - 1
					var prevTerm uint64
					if prevIdx > 0 && n.logStore != nil {
						if t, err := n.logStore.Term(prevIdx); err == nil {
							prevTerm = t
						}
					}
					if prevIdx == req.PrevLogIndex && prevTerm == req.PrevLogTerm {
						break
					}
				}
			}
		}(p.ID, req, term)
	}
}
