package kvstore

import (
	"context"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/store"
)

// 基于 Raft 的 KVStore 实现
type RaftKVStore struct {
	node *raft.Node
}

func NewRaftKVStore(node *raft.Node) store.KVStore {
	return &RaftKVStore{node: node}
}

func (r *RaftKVStore) Put(ctx context.Context, key, value string) error {
	cmd := store.Command{Op: "set", Key: key, Value: value}
	res, err := r.node.Propose(ctx, cmd)
	if err != nil {
		return err
	}
	return res.Err
}

func (r *RaftKVStore) Delete(ctx context.Context, key string) error {
	cmd := store.Command{Op: "delete", Key: key}
	res, err := r.node.Propose(ctx, cmd)
	if err != nil {
		return err
	}
	return res.Err
}

func (r *RaftKVStore) Get(ctx context.Context, key string) (string, error) {
	// 读策略有多种：
	// 1. 简单版：直接走底层 Storage 的 Get（需要 node 暴露一个 Read 接口）
	// 2. 严格线性一致读：通过 Raft 协议（可以后续扩展）
	// 这里先预留一个接口，具体实现后面考虑
	return "", nil
}
