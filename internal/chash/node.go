package chash

// 一致性哈希模式下的逻辑节点
type Node struct {
	ID      string // 本节点 ID
	Address string // 本节点地址（ClientAddress）
	Weight  int    // 环节点的权重
}

// 构造一个一致性哈希逻辑节点
func NewNode(id, address string, weight int) *Node {
	return &Node{
		ID:      id,
		Address: address,
		Weight:  weight,
	}
}
