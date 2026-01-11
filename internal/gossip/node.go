package gossip

import (
	"context"
	"distributed-kv-store/configs"
	"sync"
	"time"
)

// Gossip 节点
type Node struct {
	mu   sync.RWMutex
	self *Member

	members map[string]*Member     // 运行时的成员视图
	seeds   *[]configs.ClusterNode // 种子节点列表（配置提供）

	probeInterval  time.Duration // 探测间隔
	probeTimeout   time.Duration // 探测超时
	gossipInterval time.Duration // Gossip 传播间隔
	fanout         int           // 每轮 Gossip 传播时选择的目标节点数量
	suspectTimeout time.Duration // 节点被标记为可疑的时间
	deadTimeout    time.Duration // 节点被标记为死亡的时间

	transport Transport

	ctx     context.Context
	cancel  context.CancelFunc
	running bool

	events chan Event // 事件通道
}

// 创建新的 Gossip 节点实例
func NewNode(cfg *configs.AppConfig, transport Transport) *Node {
	member := Member{
		ID:                cfg.Self.ID,
		GossipGRPCAddress: cfg.Self.GossipGRPCAddress,
		CHashGRPCAddress:  cfg.Self.CHashGRPCAddress,
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
		seeds:          &cfg.Membership.Peers,
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
		running:        false,
	}
}

func (n *Node) Start() {
	if n == nil {
		return
	}

	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		return
	}
	// 支持 Stop 之后再次 Start：Stop 会 cancel ctx，需要重建
	if n.ctx == nil || n.cancel == nil || n.ctx.Err() != nil {
		ctx, cancel := context.WithCancel(context.Background())
		n.ctx = ctx
		n.cancel = cancel
	}

	n.running = true
	n.mu.Unlock()
	// 初始化节点内部状态，引导同步
	go n.Join(n.seeds)

	// 启动后台任务
	go n.probeLoop()
	go n.gossipLoop()
	go n.reapLoop()
}

func (n *Node) Stop() {
	if n == nil {
		return
	}

	n.mu.Lock()
	if !n.running {
		n.mu.Unlock()
		return
	}
	n.running = false
	cancel := n.cancel

	if cancel != nil {
		cancel()
	}
}
