package services

import (
	"context"
	"distributed-kv-store/internal/chash"
)

// RemoteClient 抽象“访问其他节点 KV 服务”的能力
type RemoteClient interface {
	Put(ctx context.Context, nodeAddress, key, value string) error
	Get(ctx context.Context, nodeAddress, key string) (string, error)
	Delete(ctx context.Context, nodeAddress, key string) error
}

// 基于一致性哈希的分布式 KVService 实现。
type CHashKVService struct {
	localNodeID string
	ring        *chash.HashRing

	// 本机的底层实现（可能是 raft，也可能是 standalone）
	localKV KVService

	// 远端客户端，用来调用对方的 KVService
	client RemoteClient // 例如封装 gRPC/HTTP
}

// 根据key路由到对应节点
func (s *CHashKVService) route(key string) (isLocal bool, target chash.Node) {
	node, _ := s.ring.GetNode(key)
	return node.ID == s.localNodeID, node
}

func (s *CHashKVService) Put(ctx context.Context, key, value string) error {
	isLocal, target := s.route(key)
	if isLocal {
		return s.localKV.Put(ctx, key, value)
	}
	return s.client.Put(ctx, target.Address, key, value)
}
