package bridge

import (
	"context"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/gossip"
	"distributed-kv-store/internal/storage"
	"sync"
)

// 持有 gossip 节点和一致性哈希环实例
type MemberBridge struct {
	gossipNode   *gossip.Node // 对应的 gossip 成员节点
	consHashRing chash.Ring   // 该成员所属的一致性哈希环

	transport    chash.Transport    // 内部通信
	remoteClient chash.RemoteClient // 远程客户端，用于请求转发
	st           storage.Storage    // 本地存储，独立于 service 实现内部数据通信

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// 创建新的 MemberBridge 实例
func NewMemberBridge(gossipNode *gossip.Node, ring chash.Ring, st storage.Storage) *MemberBridge {
	// gossip 节点和 ring 需要在外部组装
	ctx, cancel := context.WithCancel(context.Background())
	return &MemberBridge{
		gossipNode:   gossipNode,
		consHashRing: ring,
		st:           st,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// 启动桥接器
func (b *MemberBridge) Start() {
	if b == nil || b.gossipNode == nil || b.consHashRing == nil {
		return
	}

	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return
	}
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.running = true
	b.mu.Unlock()

	// 启动即构建一次 ring，保证有初始路由视图
	_ = b.rebuildFromSnapshot(b.gossipNode.Snapshot())

	b.wg.Add(1)

	go b.EventLoop()
}

// 停止桥接器
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

	if cancel != nil {
		cancel()
	}
	b.wg.Wait()
}
