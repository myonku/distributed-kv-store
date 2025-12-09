package raft

import "context"

// AppendEntries RPC 请求与响应

// Raft RPC 接口（供网络层实现，支持节点内部通信）
type Transport interface {
	// 发送 AppendEntries RPC
	SendAppendEntries(ctx context.Context, to string, req *AppendEntriesRequest) (*AppendEntriesResponse, error)
	// 发送 RequestVote RPC
	SendRequestVote(ctx context.Context, to string, req *RequestVoteRequest) (*RequestVoteResponse, error)
}

// AppendEntries RPC
type AppendEntriesRequest struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex uint64
	PrevLogTerm  uint64
	Entries      []LogEntry
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
	}

	// TODO: 在完整实现中，应在此处重置选举超时相关状态

	// 2. 心跳请求（无日志条目）可快速返回
	if len(req.Entries) == 0 {
		// 更新 commitIndex
		if req.LeaderCommit > n.commitIndex {
			n.commitIndex = min(req.LeaderCommit, uint64(len(n.log)))
		}
		resp.Success = true
		resp.Term = n.term
		return resp, nil
	}

	// 校验 prevLogIndex / prevLogTerm 是否匹配
	if int(req.PrevLogIndex) > len(n.log) {
		resp.Success = false
		resp.Message = "missing prevLogIndex"
		return resp, nil
	}
	if req.PrevLogIndex > 0 {
		localTerm := n.log[req.PrevLogIndex-1].Term
		if localTerm != req.PrevLogTerm {
			// 简化：直接返回失败；真实实现中应删除从 prevLogIndex 开始的冲突条目
			resp.Success = false
			resp.Message = "term mismatch at prevLogIndex"
			return resp, nil
		}
	}

	// 截断冲突之后的日志，并追加来自 Leader 的 entries
	if req.PrevLogIndex < uint64(len(n.log)) {
		n.log = n.log[:req.PrevLogIndex]
	}
	n.log = append(n.log, req.Entries...)

	// 更新 commitIndex
	lastNewIndex := uint64(len(n.log))
	if req.LeaderCommit > n.commitIndex {
		n.commitIndex = min(req.LeaderCommit, lastNewIndex)
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
	}

	// 检查候选人的日志是否至少与自己一样新
	lastIndex := uint64(len(n.log))
	var lastTerm uint64
	if lastIndex > 0 {
		lastTerm = n.log[lastIndex-1].Term
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
