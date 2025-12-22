package chash

import (
	"sync"
)

// 一致性哈希环接口
type Ring interface {
	AddNode(node Node)
	RemoveNode(nodeID string)
	GetNode(key string) (nodeID string, ok bool)
}

// 基于虚拟节点的简单一致性哈希实现
type HashRing struct {
	mu sync.RWMutex

	nodes           []Node
	VirtualNodes    int               // 每个物理节点对应的虚拟节点数
	VitrualNodesMap map[string]string // 虚拟节点到物理节点的映射
}

func NewHashRing(virtualNodes int) *HashRing {
	return &HashRing{
		VirtualNodes:    virtualNodes,
		VitrualNodesMap: make(map[string]string),
	}
}

func (r *HashRing) GetNode(key string) (node Node, ok bool) {
	return Node{}, false
}
func (r *HashRing) AddNode(node Node) {

}
func (r *HashRing) RemoveNode(nodeID string) {

}
