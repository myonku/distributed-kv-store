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
	GetNodes(key string) (nodeIDs []string, err error)
	Epoch() uint64
	RebuildWithPlan(nodes []Node) (plan RebalancePlan, err error)
}

// 用于 CHASH 层内部通信，用于副本/迁移/反熵等
type Transport interface {
	PushBatch(ctx context.Context, moveID uint32, to string, kvs *[]common.KVPair) error
	PullRange(ctx context.Context, moveID uint32, to string, startHash, endHash uint32) (*[]common.KVPair, error)
	Replicate(ctx context.Context, to string, cmds *[]common.Command) error
	AddConnection(peer configs.ClusterNode, options ...grpc.DialOption) error
	RemoveConnection(peerID string) error
	Close() error
}
