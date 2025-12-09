package services

import (
	"context"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/store/kvstore"
)

// 基于 Raft 的分布式 KVService 实现。
type RaftKVService struct {
	kv   kvstore.RaftKVStore
	node *raft.Node
}

func NewRaftKVService(kv kvstore.RaftKVStore, node *raft.Node) KVService {
	return &RaftKVService{kv: kv, node: node}
}

// 对外 Put：只在 Leader 节点接受写；非 Leader 返回错误或重定向信息
func (s *RaftKVService) Put(ctx context.Context, key, value string) error {
	if !s.node.IsLeader() {
		// 这里可以包装成更丰富的错误（带 Leader 地址）供 HTTP 层重定向
		return errors.ErrNotLeader
	}
	return s.kv.Put(ctx, key, value)
}

func (s *RaftKVService) Delete(ctx context.Context, key string) error {
	if !s.node.IsLeader() {
		return errors.ErrNotLeader
	}
	return s.kv.Delete(ctx, key)
}

func (s *RaftKVService) Get(ctx context.Context, key string) (string, error) {
	// 读策略可以有几种：
	// - 简单版：无论 Leader/Follower，直接 kv.Get
	// - 线性一致读：只允许 Leader 且通过 Raft 协议确认
	return s.kv.Get(ctx, key)
}
