package bridge

import "distributed-kv-store/internal/gossip"

// 事件循环：消费 gossip event，更新环状态并投递平衡计划
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
				plan, _ := b.rebuildFromSnapshot(ev.Snapshot)
				// 投递重建计划
				select {
				case b.balancePlanCh <- plan:
				default:
				}
			default:
				// 未知事件：保守起见也重建
				plan, _ := b.rebuildFromSnapshot(ev.Snapshot)
				// 投递重建计划
				select {
				case b.balancePlanCh <- plan:
				default:
				}
			}
		}
	}
}

// 事件循环：消费 Ring 平衡计划并执行数据迁移
func (b *MemberBridge) BalanceLoop() {
	defer b.wg.Done()

	if b == nil || b.consHashRing == nil || b.remoteClient == nil {
		return
	}

	for {
		select {
		case <-b.ctx.Done():
			return
		case plan, ok := <-b.balancePlanCh:
			if !ok {
				return
			}
			// 如果 plan 为空则跳过
			if len(plan.Moves) == 0 {
				continue
			}

			// 执行数据迁移
			for _, move := range plan.Moves {
				_ = move
			}
		}
	}
}
