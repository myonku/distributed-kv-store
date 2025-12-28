package bridge

import "distributed-kv-store/internal/gossip"

// 事件循环：消费 gossip event，更新一致性哈希环
func (b *MemberBridge) EventLoop() {
	defer b.wg.Done()

	if b == nil || b.gossipNode == nil || b.consHashRing == nil {
		return
	}

	// 默认 MemberBridge 是唯一事件消费者，所有事件需通过该桥接器处理
	// 后续可能会有其他消费者，则需要改为广播模式
	events := b.gossipNode.Events()

	for {
		select {
		case <-b.ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}

			// 可能影响 ring 的事件都触发一次全量重建。后续可能根据事件类型做增量 Add/Remove
			switch ev.Type {
			case gossip.EventMemberUp,
				gossip.EventMemberDead,
				gossip.EventMemberSuspect,
				gossip.EventMembershipChanged:
				_ = b.rebuildFromSnapshot(ev.Snapshot)
			default:
				// 未知事件：保守起见也重建
				_ = b.rebuildFromSnapshot(ev.Snapshot)
			}
		}
	}
}
