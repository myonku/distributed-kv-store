package gossip

import (
	"context"
	"distributed-kv-store/configs"
	"sync"
	"time"
)

type EventType int

const (
	EventMemberUp          EventType = iota // 新节点加入
	EventMemberSuspect                      // 节点被标记为可疑
	EventMemberDead                         // 节点被标记为死亡
	EventMembershipChanged                  // 成员信息变更
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
	member := Member{
		ID:                cfg.Self.ID,
		GossipGRPCAddress: cfg.Self.GossipGRPCAddress,
		ClientAddress:     cfg.Self.ClientAddress,
		Weight:            cfg.Self.Weight,
		State:             StateAlive,
		Incarnation:       1,
		StateUpdated:      time.Now().UnixNano(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Node{
		self:           &member,
		members:        make(map[string]*Member),
		probeInterval:  time.Duration(cfg.GossipConfig.ProbeIntervalMs) * time.Millisecond,
		probeTimeout:   time.Duration(cfg.GossipConfig.ProbeTimeoutMs) * time.Millisecond,
		gossipInterval: time.Duration(cfg.GossipConfig.GossipIntervalMs) * time.Millisecond,
		fanout:         cfg.GossipConfig.Fanout,
		suspectTimeout: time.Duration(cfg.GossipConfig.SuspectTimeoutMs) * time.Millisecond,
		deadTimeout:    time.Duration(cfg.GossipConfig.DeadTimeoutMs) * time.Millisecond,
		transport:      transport,
		events:         make(chan Event, 100),
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (n *Node) Start() {
	// TODO: 初始化节点内部状态，如引导同步等
	// 启动后台任务
	go n.probeLoop()
	go n.gossipLoop()
	go n.reapLoop()
}

func (n *Node) Stop() {
	if n == nil || n.cancel == nil {
		return
	}
	n.cancel()
}
