package chash

import "context"

// RemoteClient 抽象“访问其他节点 KV 服务”的能力
type RemoteClient interface {
	Put(ctx context.Context, nodeID, key, value string) error
	Get(ctx context.Context, nodeID, key string) (string, error)
	Delete(ctx context.Context, nodeID, key string) error
}
