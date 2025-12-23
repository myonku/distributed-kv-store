package chash

// 获取给定键对应的节点
func (r *HashRing) GetNode(key string) (node Node, ok bool) {
	return Node{}, false
}

// 添加节点并重建环
func (r *HashRing) AddNode(node Node) {
	// 可能涉及数据迁移，留待后续实现
}

// 移除节点并重建环
func (r *HashRing) RemoveNode(nodeID string) {
	// 可能涉及数据迁移，留待后续实现
}
