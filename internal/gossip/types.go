package gossip

type NodeState int
type EventType int

const (
	StateAlive   NodeState = iota // 节点存活
	StateSuspect                  // 节点可疑
	StateDead                     // 节点死亡
)

const (
	EventMemberUp          EventType = iota // 新节点加入
	EventMemberSuspect                      // 节点被标记为可疑
	EventMemberDead                         // 节点被标记为死亡
	EventMembershipChanged                  // 成员信息变更
)

// 表示一个集群成员节点的信息
type Member struct {
	ID                string // 节点 ID
	GossipGRPCAddress string // 节点间 Gossip 通信地址
	CHashGRPCAddress  string // 一致性哈希节点间通信地址
	ClientAddress     string // 对外 HTTP 地址
	Weight            int    // 环节点的权重

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

// 事件结构体
type Event struct {
	Type     EventType
	Member   Member
	Snapshot []Member // 事件发生时的成员快照
}
