package chash

import (
	"sync"

	"distributed-kv-store/configs"
)

// 一致性哈希模式下的逻辑节点
type Node struct {
	mu sync.Mutex

	id   string       // 本节点 ID
	ring Ring         // 全局 ring 视图（在所有节点上应一致）
	cli  RemoteClient // 用于访问其他节点
}

// 构造一个一致性哈希逻辑节点
func NewNode(selfID string, ring Ring, cli RemoteClient, cfg *configs.CHashClusterConfig) *Node {
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
		cli:  cli,
	}
}
