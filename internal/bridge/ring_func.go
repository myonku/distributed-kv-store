package bridge

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/errors"
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

// 根据 key 查询负责节点 ID 列表
func (b *MemberBridge) OwnerNodeIDs(key string) (nodeIDs []string, err error) {
	if b == nil || b.consHashRing == nil {
		return []string{}, errors.Error{Type: errors.ImportError, Info: "consistency hash ring not initialized"}
	}
	return b.consHashRing.GetNodes(key)
}

// ResolveReadOwners 用于读路径的 owner 解析，返回“新 owners + 旧 owners 兜底”
func (b *MemberBridge) ResolveReadOwners(key string) (primary []string, fallback []string, err error) {
	primary, fallback, err = b.consHashRing.ResolveReadOwners(key)
	if err != nil {
		return nil, nil, err
	}
	return primary, fallback, nil
}

// ResolveWriteOwners 用于写路径的 owner 解析，实现迁移窗口双写/提示写入。
func (b *MemberBridge) ResolveWriteOwners(key string) (targets []string, hinted []string, err error) {
	targets, hinted, err = b.consHashRing.ResolveWriteOwners(key)
	if err != nil {
		return nil, nil, err
	}
	return targets, hinted, nil
}

// SyncPlanHintsFromPeers 从现有成员拉取迁移计划提示（尽力而为）
func (b *MemberBridge) SyncPlanHintsFromPeers() {
	if b == nil || b.gossipNode == nil || b.consHashRing == nil {
		return
	}
	// 从当前 epoch 之后拉取提示
	sinceEpoch := b.consHashRing.Epoch()
	snapshot := b.gossipNode.Snapshot()
	selfID := b.gossipNode.SelfID()
	for _, m := range snapshot {
		if m.ID == "" || m.ID == selfID {
			continue
		}
		plans, err := b.PullPlanSince(b.ctx, m.ID, sinceEpoch)
		if err != nil || plans == nil || len(*plans) == 0 {
			continue
		}
		b.consHashRing.RecordPlanHints(plans)
	}
}
