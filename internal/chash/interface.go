package chash

import "context"

// 一致性哈希环接口
type Ring interface {
	AddNode(node Node) error
	RemoveNode(nodeID string) error
	GetNode(key string) (nodeID string, ok bool, err error)
	Rebuild(nodes []Node) error
}

// CHASH 节点远程客户端接口（业务转发）
type RemoteClient interface {
	Put(ctx context.Context, nodeID, key, value string) error
	Get(ctx context.Context, nodeID, key string) (string, error)
	Delete(ctx context.Context, nodeID, key string) error
}

// CHASH 层内部通信，用于副本/数据同步
type Transport interface {
	Replicate(ctx context.Context, nodeID string, data map[string]string) error
	PullRange(ctx context.Context, nodeID, key string, values map[string]string) error
	PushBatch(ctx context.Context, nodeID string, data map[string]string) error
}
