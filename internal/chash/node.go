package chash

// 一致性哈希模式下的逻辑节点
type Node struct {
	ID      string // 本节点 ID
	Address string // 本节点地址
	Leader  bool   // 是否为当前分片的主节点
}

// 构造一个一致性哈希逻辑节点
