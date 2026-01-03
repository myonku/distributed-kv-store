package services

import (
	"context"

	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/storage"
)

// 基于 Raft 的分布式 KVService 实现
type RaftKVService struct {
	st   storage.Storage
	node *raft.Node
}

func NewRaftKVService(st storage.Storage, node *raft.Node) KVService {
	return &RaftKVService{st: st, node: node}
}

// 只在 Leader 节点接受写；非 Leader 返回 ErrNotLeader
func (s *RaftKVService) Put(ctx context.Context, key, value string) error {
	if !s.node.IsLeader() {
		// TODO: 调用 remote client 转发请求到 Leader
		return errors.ErrNotLeader
	}

	cmd := common.Command{
		Op:    common.OpPut,
		Key:   key,
		Value: value,
	}
	_, err := s.node.Propose(ctx, cmd)
	return err
}

// 只在 Leader 上接受，其他节点返回 ErrNotLeader
func (s *RaftKVService) Delete(ctx context.Context, key string) error {
	if !s.node.IsLeader() {
		// TODO: 调用 remote client 转发请求到 Leader
		return errors.ErrNotLeader
	}

	cmd := common.Command{
		Op:  common.OpDelete,
		Key: key,
	}
	_, err := s.node.Propose(ctx, cmd)
	return err
}

// 当前实现为：只在 Leader 上允许读取，直接从本地存储读取
func (s *RaftKVService) Get(ctx context.Context, key string) (string, error) {
	if !s.node.IsLeader() {
		// 	TODO: 调用 remote client 转发请求到 Leader
		return "", errors.ErrNotLeader
	}
	// 确保线性一致性读
	if err := s.node.LinearizableRead(ctx); err != nil {
		return "", err
	}
	return s.st.Get(ctx, key)
}
