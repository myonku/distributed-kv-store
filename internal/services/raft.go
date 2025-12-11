package services

import (
	"context"

	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/storage"
)

// 基于 Raft 的分布式 KVService 实现。
//
//   - 写操作：只在 Leader 上接受，委托给 raft.Node.Propose；
//   - 读操作：当前简单实现为只允许 Leader 从本地存储读取，
//     后续可以扩展为真正的线性一致读（ReadIndex/租约读等）。
type RaftKVService struct {
	st   storage.Storage
	node *raft.Node
}

func NewRaftKVService(st storage.Storage, node *raft.Node) KVService {
	return &RaftKVService{st: st, node: node}
}

// Put 只在 Leader 节点接受写；非 Leader 返回 ErrNotLeader。
// 在 Leader 上通过 Raft 日志复制实现主从一致，当前实现未阻塞等待多数派确认，
// 主要作为调用链示意。
func (s *RaftKVService) Put(ctx context.Context, key, value string) error {
	if !s.node.IsLeader() {
		return errors.ErrNotLeader
	}

	cmd := storage.Command{
		Op:    "set", // 与 store.Storage.ApplyLog 中的语义保持一致
		Key:   key,
		Value: value,
	}
	_, err := s.node.Propose(ctx, cmd)
	return err
}

// Delete 同样只在 Leader 上接受，其他节点返回 ErrNotLeader。
func (s *RaftKVService) Delete(ctx context.Context, key string) error {
	if !s.node.IsLeader() {
		return errors.ErrNotLeader
	}

	cmd := storage.Command{
		Op:  "delete",
		Key: key,
	}
	_, err := s.node.Propose(ctx, cmd)
	return err
}

// Get 当前实现为：只在 Leader 上允许读取，直接从本地存储读取。
// 真正线性一致读通常需要通过 Raft 的 ReadIndex 或额外的
// barrier 机制保证读不会落后于已提交的写，这里留待后续扩展。
func (s *RaftKVService) Get(ctx context.Context, key string) (string, error) {
	if !s.node.IsLeader() {
		return "", errors.ErrNotLeader
	}
	return s.st.Get(ctx, key)
}
