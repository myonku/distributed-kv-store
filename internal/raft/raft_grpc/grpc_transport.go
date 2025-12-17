package raft_grpc

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"encoding/json"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 实现 Transport 接口的 gRPC 传输层
type GRPCTransport struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn  // peerID -> conn
	cli   map[string]RaftServiceClient // peerID -> client
}

// 创建 GRPCTransport，连接到所有 peers
func NewGRPCTransport(peers []configs.ClusterNode) (*GRPCTransport, error) {
	t := &GRPCTransport{
		conns: make(map[string]*grpc.ClientConn),
		cli:   make(map[string]RaftServiceClient),
	}
	for _, p := range peers {
		options := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())} // TODO: 配置凭证/超时等
		conn, err := grpc.Dial(p.InternalAddress, options...)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", p.InternalAddress, err)
		}
		t.conns[p.ID] = conn
		t.cli[p.ID] = NewRaftServiceClient(conn)
	}
	return t, nil
}

// 发送 AppendEntries RPC
func (t *GRPCTransport) SendAppendEntries(
	ctx context.Context,
	to string,
	req *raft.AppendEntriesRequest) (*raft.AppendEntriesResponse, error) {

	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return nil, errors.ErrClientNotExist
	}

	pbReq := &AppendEntriesRequest{
		Term:         req.Term,
		LeaderId:     req.LeaderID,
		PrevLogIndex: req.PrevLogIndex,
		PrevLogTerm:  req.PrevLogTerm,
		LeaderCommit: req.LeaderCommit,
		Entries:      make([]*LogEntry, 0, len(req.Entries)),
	}

	// 将 raft_store.LogEntry 映射到 proto LogEntry
	for _, e := range req.Entries {
		cmdData, err := json.Marshal(e.Cmd)
		if err != nil {
			return nil, fmt.Errorf("marshal command: %w", err)
		}

		var confData []byte
		if e.Conf != nil {
			confData, err = json.Marshal(e.Conf)
			if err != nil {
				return nil, fmt.Errorf("marshal conf: %w", err)
			}
		}

		pbReq.Entries = append(pbReq.Entries, &LogEntry{
			Index:   e.Index,
			Term:    e.Term,
			CmdData: cmdData,
			Type:    uint32(e.Type),
			Conf:    confData,
		})
	}

	pbResp, err := client.AppendEntries(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &raft.AppendEntriesResponse{
		Term:    pbResp.Term,
		Success: pbResp.Success,
		Message: pbResp.Message,
	}, nil
}

// 发送 RequestVote RPC
func (t *GRPCTransport) SendRequestVote(
	ctx context.Context,
	to string,
	req *raft.RequestVoteRequest) (*raft.RequestVoteResponse, error) {

	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return nil, errors.ErrClientNotExist
	}

	pbReq := &RequestVoteRequest{
		Term:         req.Term,
		CandidateId:  req.CandidateID,
		LastLogIndex: req.LastLogIndex,
		LastLogTerm:  req.LastLogTerm,
	}

	pbResp, err := client.RequestVote(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &raft.RequestVoteResponse{
		Term:        pbResp.Term,
		VoteGranted: pbResp.VoteGranted,
	}, nil
}

// 添加集群节点
func (t *GRPCTransport) AddPeer(peer configs.ClusterNode) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.cli[peer.ID]; exists {
		// 已存在，覆盖连接
		_ = t.conns[peer.ID].Close()
		delete(t.conns, peer.ID)
		delete(t.cli, peer.ID)
	}
	options := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())} // TODO: 配置凭证/超时等
	conn, err := grpc.Dial(peer.InternalAddress, options...)
	if err != nil {
		return fmt.Errorf("dial %s: %w", peer.InternalAddress, err)
	}
	t.conns[peer.ID] = conn
	t.cli[peer.ID] = NewRaftServiceClient(conn)
	return nil
}

// 移除集群节点
func (t *GRPCTransport) RemovePeer(peerID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	conn, exists := t.conns[peerID]
	if !exists {
		return nil // 不存在，直接返回
	}
	err := conn.Close()
	if err != nil {
		return err
	}
	delete(t.conns, peerID)
	delete(t.cli, peerID)
	return nil
}

func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range t.conns {
		_ = c.Close()
	}
	return nil
}
