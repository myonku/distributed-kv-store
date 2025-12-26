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

	ringKeys    []uint32          // 保存所有虚拟节点的 hash 值并保持有序，用于 GetNode 二分查找
	vnodeOwners map[uint32]string // vnodeOwners 将虚拟节点 hash 映射到物理节点 ID
}

// 返回空的一致性哈希环实例，实际调用时根据 Gossip 成员动态构建
func NewHashRing(cfg *configs.AppConfig) *HashRing {
	return &HashRing{
		VirtualNodes:    cfg.CHash.VirtualNodes,
		VitrualNodesMap: make(map[string]string),
		vnodeOwners:     make(map[uint32]string),
	}
}
