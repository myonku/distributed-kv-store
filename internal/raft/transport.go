package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/raft/raft_store"
)

// AppendEntries RPC 请求与响应

// Raft RPC 接口（供网络层实现，支持节点内部通信）
type Transport interface {
	// 发送 AppendEntries RPC
	SendAppendEntries(ctx context.Context, to string, req *AppendEntriesRequest) (*AppendEntriesResponse, error)
	// 发送 RequestVote RPC
	SendRequestVote(ctx context.Context, to string, req *RequestVoteRequest) (*RequestVoteResponse, error)
	// 添加新的集群节点连接（本地）
	AddPeer(peer configs.ClusterNode) error
	// 移除某个集群节点的连接（本地）
	RemovePeer(peerID string) error
}

// AppendEntries RPC
type AppendEntriesRequest struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []raft_store.LogEntry
	LeaderCommit uint64
}

// AppendEntries RPC 响应
type AppendEntriesResponse struct {
	Term    uint64
	Success bool
	Message string
}

// RequestVote RPC
type RequestVoteRequest struct {
	Term         uint64
	CandidateID  string
	LastLogIndex uint64
	LastLogTerm  uint64
}

// RequestVote RPC 响应
type RequestVoteResponse struct {
	Term        uint64
	VoteGranted bool
}

// 处理来自其他节点的 AppendEntries（由 RPC 层调用）
func (n *Node) HandleAppendEntries(ctx context.Context, req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	resp := &AppendEntriesResponse{}

	n.mu.Lock()
	defer n.mu.Unlock()

	resp.Term = n.term

	// 如果请求 term 过小，直接拒绝
	if req.Term < n.term {
		resp.Success = false
		resp.Message = "term too old"
		return resp, nil
	}

	// 如果收到更高 term，更新本地 term 并退回 Follower
	if req.Term > n.term {
		n.term = req.Term
		n.role = Follower
		n.votedFor = ""
		n.hardStateStore.Save(raft_store.HardState{
			Term:        n.term,
			VotedFor:    n.votedFor,
			CommitIndex: n.commitIndex,
		})
	}

	// 重置选举超时
	n.resetElectionTimeout()

	// 心跳请求（无日志条目）可快速返回
	if len(req.Entries) == 0 {
		// 更新 commitIndex
		if req.LeaderCommit > n.commitIndex {
			var lastIndex uint64
			if n.logStore != nil {
				if li, err := n.logStore.LastIndex(); err == nil {
					lastIndex = li
				}
			}
			n.commitIndex = min(req.LeaderCommit, lastIndex)
			n.hardStateStore.Save(raft_store.HardState{
				Term:        n.term,
				VotedFor:    n.votedFor,
				CommitIndex: n.commitIndex,
			})
		}
		resp.Success = true
		resp.Term = n.term
		return resp, nil
	}

	// 校验 prevLogIndex / prevLogTerm 是否匹配
	var lastIndex uint64
	if n.logStore != nil {
		if li, err := n.logStore.LastIndex(); err == nil {
			lastIndex = li
		}
	}
	if req.PrevLogIndex > lastIndex {
		resp.Success = false
		resp.Message = "missing prevLogIndex"
		return resp, nil
	}
	if req.PrevLogIndex > 0 {
		var localTerm uint64
		if n.logStore != nil {
			if t, err := n.logStore.Term(req.PrevLogIndex); err == nil {
				localTerm = t
			}
		}
		if localTerm != req.PrevLogTerm {
			// 简化：删除从 prevLogIndex 开始的冲突条目
			resp.Success = false
			resp.Message = "term mismatch at prevLogIndex"
			return resp, nil
		}
	}

	// 截断冲突之后的日志，并追加来自 Leader 的 entries
	if n.logStore != nil {
		// 从 PrevLogIndex 之后的位置开始截断，即删除 [PrevLogIndex+1, ...]
		if err := n.logStore.TruncateFrom(req.PrevLogIndex + 1); err != nil {
			resp.Success = false
			resp.Message = "truncate log failed"
			return resp, nil
		}
		if err := n.logStore.Append(req.Entries); err != nil {
			resp.Success = false
			resp.Message = "append log failed"
			return resp, nil
		}
	}

	// 更新 commitIndex
	lastNewIndex := lastIndex
	if n.logStore != nil {
		if li, err := n.logStore.LastIndex(); err == nil {
			lastNewIndex = li
		}
	}
	if req.LeaderCommit > n.commitIndex {
		n.commitIndex = min(req.LeaderCommit, lastNewIndex)
		n.hardStateStore.Save(raft_store.HardState{
			Term:        n.term,
			VotedFor:    n.votedFor,
			CommitIndex: n.commitIndex,
		})
	}

	resp.Success = true
	resp.Term = n.term
	return resp, nil
}

// 处理 RequestVote 请求（由 RPC 层调用）
func (n *Node) HandleRequestVote(ctx context.Context, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	resp := &RequestVoteResponse{}

	n.mu.Lock()
	defer n.mu.Unlock()

	resp.Term = n.term

	// 如果请求 term 过小，拒绝
	if req.Term < n.term {
		resp.VoteGranted = false
		return resp, nil
	}

	// 如果请求 term 更大，更新本地 term 并退回 Follower
	if req.Term > n.term {
		n.term = req.Term
		n.role = Follower
		n.votedFor = ""
		n.hardStateStore.Save(raft_store.HardState{
			Term:        n.term,
			VotedFor:    n.votedFor,
			CommitIndex: n.commitIndex,
		})
	}

	// 检查候选人的日志是否至少与自己一样新
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

	upToDate := false
	if req.LastLogTerm > lastTerm {
		upToDate = true
	} else if req.LastLogTerm == lastTerm && req.LastLogIndex >= lastIndex {
		upToDate = true
	}

	// 如果尚未投票或已投给该候选人，且日志足够新，则投票
	if (n.votedFor == "" || n.votedFor == req.CandidateID) && upToDate {
		n.votedFor = req.CandidateID
		resp.VoteGranted = true
	}

	resp.Term = n.term
	return resp, nil
}
