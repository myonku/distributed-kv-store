package chash

import "context"

// 一致性哈希环接口
type Ring interface {
	AddNode(node Node) error
	RemoveNode(nodeID string) error
	GetNode(key string) (nodeID string, ok bool, err error)
	Rebuild(nodes []Node) error
}

// 用于节点间批量数据交换（副本、迁移等）
type KVPair struct {
	Key   string
	Value string
}

// 用于业务转发（data-plane），调用目标节点的对外 HTTP 接口
type RemoteClient interface {
	Put(ctx context.Context, targetAddr, key, value string) error
	Get(ctx context.Context, targetAddr, key string) (string, error)
	Delete(ctx context.Context, targetAddr, key string) error
}

// 用于 CHASH 层内部通信（control-plane），用于副本/迁移/反熵等
type Transport interface {
	PushBatch(ctx context.Context, to string, kvs []KVPair) error
	PullRange(ctx context.Context, to string, startKey, endKey string) ([]KVPair, error)
	Replicate(ctx context.Context, to string, kvs []KVPair) error
}
