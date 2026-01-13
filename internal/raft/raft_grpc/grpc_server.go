package raft_grpc

import (
	"context"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/raft/raft_store"
	"encoding/json"
	"fmt"
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
		entryType := common.LogEntryType(e.Type)
		internalEntry := raft_store.LogEntry{
			Index: e.Index,
			Term:  e.Term,
			Type:  entryType,
		}

		switch entryType {
		case common.EntryNormal:
			if len(e.CmdData) > 0 {
				var cmd common.Command
				if err := json.Unmarshal(e.CmdData, &cmd); err != nil {
					return nil, errors.Error{Type: errors.InternalError, Info: fmt.Sprintf("unmarshal command: %v", err)}
				}
				internalEntry.Cmd = cmd
			}

		case common.EntryConfChange:
			if len(e.Conf) == 0 {
				return nil, errors.Error{Type: errors.InvalidArgument, Info: fmt.Sprintf("missing conf for conf-change entry (index=%d)", e.Index)}
			}
			var cc common.ClusterConfigChange
			if err := json.Unmarshal(e.Conf, &cc); err != nil {
				return nil, errors.Error{Type: errors.InternalError, Info: fmt.Sprintf("unmarshal conf change: %v", err)}
			}
			internalEntry.Conf = &cc

		default:
			return nil, errors.Error{Type: errors.InvalidArgument, Info: "invalid log entry type"}
		}

		internalReq.Entries = append(internalReq.Entries, internalEntry)
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

// 处理 RequestVote RPC 调用
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
