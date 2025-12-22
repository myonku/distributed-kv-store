package gossip

import (
	"context"
	"distributed-kv-store/configs"
	"sync"
	"time"
)

type EventType int

const (
	EventMemberUp EventType = iota
	EventMemberSuspect
	EventMemberDead
	EventMembershipChanged
)

type Event struct {
	Type     EventType
	Member   Member
	Snapshot []Member // 事件发生时的成员快照
}

// Gossip 节点
type Node struct {
	mu   sync.RWMutex
	self *Member

	members map[string]*Member

	probeInterval  time.Duration // 探测间隔
	probeTimeout   time.Duration // 探测超时
	gossipInterval time.Duration // Gossip 传播间隔
	fanout         int           // 每轮 Gossip 传播时选择的目标节点数量
	suspectTimeout time.Duration // 节点被标记为可疑的时间
	deadTimeout    time.Duration // 节点被标记为死亡的时间

	transport Transport

	ctx    context.Context
	cancel context.CancelFunc

	events chan Event // 事件通道
}

// 创建新的 Gossip 节点实例
func NewNode(cfg *configs.AppConfig, transport Transport) *Node {
	menber := Member{
		ID:              cfg.Self.ID,
		InternalAddress: cfg.Self.InternalAddress,
		ClientAddress:   cfg.Self.ClientAddress,
		Weight:          cfg.Self.Weight,
		State:           StateAlive,
		Incarnation:     1,
		StateUpdated:    time.Now().UnixNano(),
	}
	return &Node{
		self:           &menber,
		members:        make(map[string]*Member),
		probeInterval:  time.Duration(cfg.GossipConfig.ProbeIntervalMs),
		probeTimeout:   time.Duration(cfg.GossipConfig.ProbeTimeoutMs),
		gossipInterval: time.Duration(cfg.GossipConfig.GossipIntervalMs),
		fanout:         cfg.GossipConfig.Fanout,
		suspectTimeout: time.Duration(cfg.GossipConfig.SuspectTimeoutMs),
		deadTimeout:    time.Duration(cfg.GossipConfig.DeadTimeoutMs),
		transport:      transport,
		events:         make(chan Event, 100),
	}
}

func (n *Node) Start() {

}
func (n *Node) Stop() {

}

// Join 用于启动时引导（seed 节点列表来自配置）
func (n *Node) Join(seeds []configs.ClusterNode) error {
	return nil
}

func (n *Node) Snapshot() []Member {
	return nil
}
func (n *Node) Events() <-chan Event {
	return n.events
}
