package gossip

type NodeState int

const (
	StateAlive NodeState = iota
	StateSuspect
	StateDead
)

// 表示一个集群成员节点的信息
type Member struct {
	ID                string
	GossipGRPCAddress string
	ClientAddress     string
	Weight            int // 环节点的权重

	Incarnation  uint64    // 每个节点自己的版本号（或每条记录带版本）
	StateUpdated int64     // unix nano，便于超时判断
	State        NodeState // 节点状态
}

// 节点摘要信息（用于 Gossip 消息传播）
type Digest struct {
	ID          string
	Incarnation uint64
	State       NodeState
}
