package bridge

import (
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/gossip"
)

// 提取 gossip 成员信息为 chash 节点
func MemberToNode(m *gossip.Member) *chash.Node {
	return chash.NewNode(m.ID, m.ClientAddress, m.Weight)
}

// 批量转换 gossip 成员为 chash 节点列表
func MembersToNodes(members []gossip.Member) []chash.Node {
	nodes := make([]chash.Node, 0, len(members))
	for _, m := range members {
		nodes = append(nodes, *MemberToNode(&m))
	}
	return nodes
}

// 事件循环：消费 gossip event，更新一致性哈希环
func (b *MemberBridge) EventLoop() {
	defer b.wg.Done()

	if b == nil || b.GossipMember == nil || b.ConsHashRing == nil {
		return
	}

	// 默认 MemberBridge 是唯一事件消费者
	// 后续可能会有其他消费者，则需要改为广播模式
	events := b.GossipMember.Events()

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
			case gossip.EventMemberUp, gossip.EventMemberDead, gossip.EventMemberSuspect, gossip.EventMembershipChanged:
				_ = b.rebuildFromSnapshot(ev.Snapshot)
			default:
				// 未知事件：保守起见也重建
				_ = b.rebuildFromSnapshot(ev.Snapshot)
			}
		}
	}
}

func (b *MemberBridge) rebuildFromSnapshot(snapshot []gossip.Member) error {
	if b == nil || b.ConsHashRing == nil {
		return nil
	}
	// 排除 Dead，其余都可以进 ring
	aliveOrSuspect := make([]gossip.Member, 0, len(snapshot))
	for _, m := range snapshot {
		if m.State == gossip.StateDead {
			continue
		}
		aliveOrSuspect = append(aliveOrSuspect, m)
	}
	return b.ConsHashRing.Rebuild(MembersToNodes(aliveOrSuspect))
}
