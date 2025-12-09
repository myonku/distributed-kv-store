package services

import (
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/store"
)

// 基于一致性哈希的分布式 KVService 实现。
type CHashKVService struct {
	kv   store.KVStore
	node *chash.Node
}

func NewCHashKVService(kv store.KVStore, node *chash.Node) KVService
