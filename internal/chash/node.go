package chash

import (
	"context"
	"errors"
	"sync"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/store"
)

// 一致性哈希模式下的逻辑节点
type Node struct {
	mu sync.RWMutex

	id   string        // 本节点 ID
	ring Ring          // 全局 ring 视图（在所有节点上应一致）
	kv   store.KVStore // 本机状态机/存储
	cli  RemoteClient  // 用于访问其他节点
}

// 构造一个一致性哈希逻辑节点
func NewNode(selfID string, kv store.KVStore, ring Ring, cli RemoteClient, cfg *configs.CHashClusterConfig) *Node {
	// 初始化 ring 中的节点
	if cfg != nil {
		for _, n := range cfg.Nodes {
			weight := n.Weight
			if weight <= 0 {
				weight = 1
			}
			ring.AddNode(n.ID, weight)
		}
	}

	return &Node{
		id:   selfID,
		ring: ring,
		kv:   kv,
		cli:  cli,
	}
}

func (n *Node) Put(ctx context.Context, key, value string) error {
	targetID, ok := n.ring.GetNode(key)
	if !ok {
		return errors.New("no node for key")
	}

	if targetID == n.id {
		// 本机写入
		return n.kv.Put(ctx, key, value)
	}

	// 远程写入
	if n.cli == nil {
		return errors.New("no remote client configured")
	}
	return n.cli.Put(ctx, targetID, key, value)
}

func (n *Node) Get(ctx context.Context, key string) (string, error) {
	targetID, ok := n.ring.GetNode(key)
	if !ok {
		return "", errors.New("no node for key")
	}

	if targetID == n.id {
		return n.kv.Get(ctx, key)
	}

	if n.cli == nil {
		return "", errors.New("no remote client configured")
	}
	return n.cli.Get(ctx, targetID, key)
}

func (n *Node) Delete(ctx context.Context, key string) error {
	targetID, ok := n.ring.GetNode(key)
	if !ok {
		return errors.New("no node for key")
	}

	if targetID == n.id {
		return n.kv.Delete(ctx, key)
	}

	if n.cli == nil {
		return errors.New("no remote client configured")
	}
	return n.cli.Delete(ctx, targetID, key)
}
