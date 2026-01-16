package bridge

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/gossip"
)

// 从 gossip 成员快照重建一致性哈希环并维护通信信息，返回 Ring 重建计划
func (b *MemberBridge) rebuildFromSnapshot(snapshot []gossip.Member) (chash.MovePlan, error) {
	if b == nil || b.consHashRing == nil {
		return chash.MovePlan{}, nil
	}

	// 获取本节点 ID 以避免自连接
	selfID := ""
	if b.gossipNode != nil {
		selfID = b.gossipNode.SelfID()
	}

	seen := make(map[string]struct{}, len(snapshot))
	// 排除 Dead，其余都可以进 ring
	aliveOrSuspect := make([]gossip.Member, 0, len(snapshot))
	for _, m := range snapshot {
		seen[m.ID] = struct{}{}

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
		var oldInfo MemberAddrInfo
		oldVal, hadOld := b.memberAddrs.Load(m.ID)
		if hadOld {
			if oi, ok := oldVal.(MemberAddrInfo); ok {
				oldInfo = oi
			} else {
				hadOld = false
			}
		}

		newInfo := MemberAddrInfo{
			clientAddress:    m.ClientAddress,
			chashGRPCAddress: m.CHashGRPCAddress,
		}
		b.memberAddrs.Store(m.ID, newInfo)

		// 更新 transport 连接信息：只在新增或地址变更时重连
		if b.transport != nil && m.ID != selfID {
			needReconnect := !hadOld || oldInfo.chashGRPCAddress != newInfo.chashGRPCAddress
			if needReconnect {
				cc := configs.ClusterNode{
					ID:               m.ID,
					CHashGRPCAddress: m.CHashGRPCAddress,
					ClientAddress:    m.ClientAddress,
				}
				_ = b.transport.AddConnection(cc)
			}
		}
		aliveOrSuspect = append(aliveOrSuspect, m)
	}

	// snapshot 里不存在但 memberAddrs/transport 里可能还有残留
	b.memberAddrs.Range(func(k, _ any) bool {
		id, ok := k.(string)
		if !ok {
			return true
		}
		if _, ok := seen[id]; ok {
			return true
		}
		b.memberAddrs.Delete(id)
		if b.transport != nil {
			_ = b.transport.RemoveConnection(id)
		}
		return true
	})

	// 重建一致性哈希环
	plan, err := b.consHashRing.RebuildWithPlan(MembersToNodes(aliveOrSuspect))
	if err == nil {
		return plan, nil
	}

	return chash.MovePlan{}, err
}
