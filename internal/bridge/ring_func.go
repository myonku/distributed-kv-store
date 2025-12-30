package bridge

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/gossip"
)

// 提取 gossip 成员信息为 chash 节点
func MemberToNode(m *gossip.Member) *chash.Node {
	return chash.NewNode(
		m.ID,
		m.ClientAddress,
		m.CHashGRPCAddress,
		m.Weight)
}

// 批量转换 gossip 成员为 chash 节点列表
func MembersToNodes(members []gossip.Member) []chash.Node {
	nodes := make([]chash.Node, 0, len(members))
	for _, m := range members {
		nodes = append(nodes, *MemberToNode(&m))
	}
	return nodes
}

// 从 gossip 成员快照重建一致性哈希环并维护通信信息
// 暂未实现增量更新
func (b *MemberBridge) rebuildFromSnapshot(snapshot []gossip.Member) error {
	if b == nil || b.consHashRing == nil {
		return nil
	}
	// 排除 Dead，其余都可以进 ring
	aliveOrSuspect := make([]gossip.Member, 0, len(snapshot))
	for _, m := range snapshot {
		if m.State == gossip.StateDead {
			// dead 直接从地址表移除
			b.memberAddrs.Delete(m.ID)
			// 更新transport连接信息
			if b.transport != nil {
				_ = b.transport.RemoveConnection(m.ID)
			}
			continue
		}
		// 维护节点地址信息，用于后续业务转发/内部通信
		b.memberAddrs.Store(m.ID, MemberAddrInfo{
			clientAddress:    m.ClientAddress,
			chashGRPCAddress: m.CHashGRPCAddress,
		})
		// 更新transport连接信息
		if b.transport != nil {
			cc := configs.ClusterNode{
				ID:               m.ID,
				CHashGRPCAddress: m.CHashGRPCAddress,
				ClientAddress:    m.ClientAddress,
			}
			_ = b.transport.AddConnection(cc)
		}
		aliveOrSuspect = append(aliveOrSuspect, m)
	}
	return b.consHashRing.Rebuild(MembersToNodes(aliveOrSuspect))
}
