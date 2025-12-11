package raft_rpc

import (
	context "context"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/raft/raft_store"
	"distributed-kv-store/internal/storage"
)

// RaftServiceServer 的实现，内部持有 *Node
type RaftGRPCServer struct {
	UnimplementedRaftServiceServer
	node *raft.Node
}

func NewRaftGRPCServer(node *raft.Node) *RaftGRPCServer {
	return &RaftGRPCServer{node: node}
}

// 处理 AppendEntries RPC 调用
func (s *RaftGRPCServer) AppendEntries(ctx context.Context, req *AppendEntriesRequest) (*AppendEntriesResponse, error) {
	// 将 proto 消息转为内部 raft.AppendEntriesRequest
	internalReq := &raft.AppendEntriesRequest{
		Term:         req.Term,
		LeaderID:     req.LeaderId,
		PrevLogIndex: req.PrevLogIndex,
		PrevLogTerm:  req.PrevLogTerm,
		Entries:      make([]raft_store.LogEntry, 0, len(req.Entries)),
		LeaderCommit: req.LeaderCommit,
	}

	for _, e := range req.Entries {
		var cmd storage.Command
		internalReq.Entries = append(internalReq.Entries, raft_store.LogEntry{
			Index: e.Index,
			Term:  e.Term,
			Cmd:   cmd,
		})
	}

	// 调用 Node 的 HandleAppendEntries
	resp, err := s.node.HandleAppendEntries(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	// 再转换回 proto 响应
	return &AppendEntriesResponse{
		Term:    resp.Term,
		Success: resp.Success,
		Message: resp.Message,
	}, nil
}

func (s *RaftGRPCServer) RequestVote(ctx context.Context, req *RequestVoteRequest) (*RequestVoteResponse, error) {
	internalReq := &raft.RequestVoteRequest{
		Term:         req.Term,
		CandidateID:  req.CandidateId,
		LastLogIndex: req.LastLogIndex,
		LastLogTerm:  req.LastLogTerm,
	}

	// 调用 Node 的 HandleRequestVote
	resp, err := s.node.HandleRequestVote(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return &RequestVoteResponse{
		Term:        resp.Term,
		VoteGranted: resp.VoteGranted,
	}, nil
}
