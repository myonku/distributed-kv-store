package services

import (
	"context"
	"distributed-kv-store/internal/bridge"
	"distributed-kv-store/internal/storage"
)

// 基于Gossip + 一致性哈希模式的分布式 KVService 实现
type CHashKVService struct {
	memberBridge *bridge.MemberBridge // 持有 gossip 节点和一致性哈希环实例
	st           storage.Storage      // 本地存储
}

func (s *CHashKVService) Put(ctx context.Context, key, value string) error {
	return nil
}

func (s *CHashKVService) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (s *CHashKVService) Delete(ctx context.Context, key string) error {
	return nil
}
