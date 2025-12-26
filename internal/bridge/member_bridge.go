package bridge

import (
	"context"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/gossip"
	"sync"
)

// 持有 gossip 节点和一致性哈希环实例
type MemberBridge struct {
	GossipMember *gossip.Node // 对应的 gossip 成员节点
	ConsHashRing chash.Ring   // 该成员所属的一致性哈希环

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

func NewMemberBridge(gossipMember *gossip.Node, ring chash.Ring) *MemberBridge {
	return &MemberBridge{
		GossipMember: gossipMember,
		ConsHashRing: ring,
	}
}

// 启动桥接器
func (b *MemberBridge) Start() {
	if b == nil || b.GossipMember == nil || b.ConsHashRing == nil {
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

	// 启动即构建一次 ring，保证有初始路由视图。
	_ = b.rebuildFromSnapshot(b.GossipMember.Snapshot())

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
