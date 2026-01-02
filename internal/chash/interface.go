package chash

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/common"

	"google.golang.org/grpc"
)

// 一致性哈希环接口
type Ring interface {
	GetNode(key string) (nodeID string, ok bool, err error)
	Rebuild(nodes []Node) error
	RebuildWithPlan(nodes []Node) (plan RebalancePlan, err error)
}

// 用于业务转发，调用目标节点的对外 HTTP 接口
type RemoteClient interface {
	Put(ctx context.Context, targetAddr, key, value string) error
	Get(ctx context.Context, targetAddr, key string) (string, error)
	Delete(ctx context.Context, targetAddr, key string) error
}

// 用于 CHASH 层内部通信，用于副本/迁移/反熵等
type Transport interface {
	PushBatch(ctx context.Context, to string, kvs *[]common.KVPair) error
	PullRange(ctx context.Context, to string, startHash, endHash uint32) (*[]common.KVPair, error)
	Replicate(ctx context.Context, to string, cmds *[]common.Command) error
	AddConnection(peer configs.ClusterNode, options ...grpc.DialOption) error
	RemoveConnection(peerID string) error
	Close() error
}
