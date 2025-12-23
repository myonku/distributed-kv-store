package chash

import (
	"distributed-kv-store/configs"
	"sync"
)

// 基于虚拟节点的简单一致性哈希实现
type HashRing struct {
	mu sync.RWMutex

	nodes           []Node
	VirtualNodes    int               // 每个物理节点对应的虚拟节点数
	VitrualNodesMap map[string]string // 虚拟节点到物理节点的映射
}

func NewHashRing(cfg *configs.AppConfig) *HashRing {
	ring := &HashRing{
		VirtualNodes:    cfg.CHash.VirtualNodes,
		VitrualNodesMap: make(map[string]string),
	}
	// 根据配置添加初始节点
	for _, member := range cfg.Membership.Peers {
		node := Node{
			ID:      member.ID,
			Address: member.ClientAddress,
			Weight:  member.Weight,
		}
		ring.AddNode(node)
	}
	return ring
}
