package services

import (
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/store"
)

// 基于 Raft 主从模式下的 KVService 实现。
type RaftKVService struct {
	kv   store.KVStore
	node *raft.Node
}

func NewRaftKVService(kv store.KVStore, node *raft.Node) KVService
