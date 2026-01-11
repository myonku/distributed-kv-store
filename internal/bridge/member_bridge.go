package bridge

import (
	"context"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/gossip"
	"distributed-kv-store/internal/storage"
	"sync"
)

// 成员节点地址信息
type MemberAddrInfo struct {
	clientAddress    string // 对外服务地址
	chashGRPCAddress string // 节点间通信地址（内部通信暂时由 transport 维护）
}

// 持有 gossip 节点和一致性哈希环实例
type MemberBridge struct {
	gossipNode   *gossip.Node // 对应的 gossip 成员节点
	consHashRing chash.Ring   // 该成员所属的一致性哈希环

	memberAddrs  *sync.Map           // 节点ID->地址信息映射
	transport    chash.Transport     // 内部通信
	remoteClient common.RemoteClient // 远程客户端，用于请求转发
	st           storage.Storage     // 用于支持数据迁移及记录持久化

	snapshotCh       chan []gossip.Member // 成员快照更新通道（用于驱动 ring rebuild + rebalance）
	lastAppliedEpoch uint64               // 上次应用的 Ring 版本
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	running          bool
}

// 创建新的 MemberBridge 实例
func NewMemberBridge(
	gossipNode *gossip.Node,
	ring chash.Ring,
	chashTransport chash.Transport,
	remoteClient common.RemoteClient,
	st storage.Storage,
) *MemberBridge {

	// gossip 节点和 ring 需要在外部组装
	ctx, cancel := context.WithCancel(context.Background())
	return &MemberBridge{
		gossipNode:       gossipNode,
		consHashRing:     ring,
		ctx:              ctx,
		cancel:           cancel,
		transport:        chashTransport,
		remoteClient:     remoteClient,
		memberAddrs:      &sync.Map{},
		st:               st,
		snapshotCh:       make(chan []gossip.Member, 1),
		lastAppliedEpoch: ring.Epoch(),
		running:          false,
	}
}

// 启动 MemberBridge 的后台任务
func (b *MemberBridge) Start() {
	if b == nil || b.gossipNode == nil || b.consHashRing == nil {
		return
	}
	// 先尝试启动内部节点
	if b.gossipNode != nil && !b.gossipNode.IsRunning() {
		b.gossipNode.Start()
	}

	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	// 支持 Stop 之后再次 Start：Stop 会 cancel ctx，需要重建
	if b.ctx == nil || b.cancel == nil || b.ctx.Err() != nil {
		ctx, cancel := context.WithCancel(context.Background())
		b.ctx = ctx
		b.cancel = cancel
	}
	b.running = true
	b.mu.Unlock()

	b.wg.Add(2)
	go b.runEventLoop()
	go b.runBalanceLoop()

	// 启动即投递一次快照，保证有初始路由视图
	b.enqueueSnapshot(b.gossipNode.Snapshot())
}

func (b *MemberBridge) Stop() {
	if b == nil {
		return
	}

	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	cancel := b.cancel
	b.running = false
	b.mu.Unlock()

	// 停止 gossip 节点
	if b.gossipNode != nil && b.gossipNode.IsRunning() {
		b.gossipNode.Stop()
	}
	// 关闭 transport
	if b.transport != nil {
		b.transport.Close()
	}

	if cancel != nil {
		cancel()
	}
	b.wg.Wait()
}
