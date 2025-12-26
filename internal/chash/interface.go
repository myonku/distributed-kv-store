package chash

import "context"

// 一致性哈希环接口
type Ring interface {
	AddNode(node Node) error
	RemoveNode(nodeID string) error
	GetNode(key string) (nodeID string, ok bool, err error)
	Rebuild(nodes []Node) error
}

// 为 CHASH 节点定义远程客户端接口
type RemoteClient interface {
	Put(ctx context.Context, nodeID, key, value string) error
	Get(ctx context.Context, nodeID, key string) (string, error)
	Delete(ctx context.Context, nodeID, key string) error
}
