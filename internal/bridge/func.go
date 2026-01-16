package bridge

import (
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
)

// 返回自身节点 ID
func (b *MemberBridge) SelfID() string {
	if b == nil || b.gossipNode == nil {
		return ""
	}
	return b.gossipNode.SelfID()
}

// 返回 MemberBridge 是否正在运行
func (b *MemberBridge) IsRunning() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

// 根据 key 查询负责节点 ID
func (b *MemberBridge) OwnerNodeID(key string) (nodeID string, ok bool, err error) {
	if b == nil || b.consHashRing == nil {
		return "", false, errors.Error{Type: errors.ImportError, Info: "consistency hash ring not initialized"}
	}
	return b.consHashRing.GetNode(key)
}

// 根据 key 查询负责节点 ID 列表
func (b *MemberBridge) OwnerNodeIDs(key string) (nodeIDs []string, err error) {
	if b == nil || b.consHashRing == nil {
		return []string{}, errors.Error{Type: errors.ImportError, Info: "consistency hash ring not initialized"}
	}
	return b.consHashRing.GetNodes(key)
}

// ResolveReadOwners 用于读路径的 owner 解析（占位）：
// 后续可结合 plan hints 返回“新 owners + 旧 owners 兜底”。
func (b *MemberBridge) ResolveReadOwners(key string) (primary []string, fallback []string, err error) {
	nodes, err := b.OwnerNodeIDs(key)
	if err != nil {
		return nil, nil, err
	}
	return nodes, nil, nil
}

// ResolveWriteOwners 用于写路径的 owner 解析（占位）：
// 后续可结合 plan hints 实现迁移窗口双写/提示写入。
func (b *MemberBridge) ResolveWriteOwners(key string) (targets []string, hinted []string, err error) {
	nodes, err := b.OwnerNodeIDs(key)
	if err != nil {
		return nil, nil, err
	}
	return nodes, nil, nil
}

// 提取 gossip 成员信息为 chash 节点
func MemberToNode(m *gossip.Member) *chash.Node {
	return chash.NewNode(
		m.ID,
		m.ClientAddress,
		m.CHashGRPCAddress,
		m.Weight,
	)
}

// 批量转换 gossip 成员为 chash 节点列表
func MembersToNodes(members []gossip.Member) []chash.Node {
	nodes := make([]chash.Node, 0, len(members))
	for _, m := range members {
		nodes = append(nodes, *MemberToNode(&m))
	}
	return nodes
}

// RecordPlanHints 本地记录迁移计划提示（旧/新 owner 集合、状态），用于读写兜底
func (b *MemberBridge) RecordPlanHints(plan chash.MovePlan) error {
	if b == nil || b.consHashRing == nil || len(plan.Hints) == 0 {
		return errors.Error{Type: errors.ImportError, Info: "consistency hash ring not initialized or no hints"}
	}
	b.consHashRing.RecordPlanHints(&plan.Hints)
	return nil
}

// LookupPlanHintForHash 查询给定哈希是否命中迁移计划提示，返回旧/新 owner 集合及版本
func (b *MemberBridge) LookupPlanHintForHash(hash uint32) (oldOwners []string, newOwners []string, epoch uint64, ok bool) {
	if b == nil || b.consHashRing == nil {
		return nil, nil, 0, false
	}
	hint, ok := b.consHashRing.LookupPlanHintForHash(hash)
	if !ok {
		return nil, nil, 0, false
	}
	return hint.OldOwners, hint.NewOwners, hint.Epoch, true
}

// UpdatePlanHintStatus 更新计划提示状态（planned/in-progress/completed），用于读写策略决策
func (b *MemberBridge) UpdatePlanHintStatus(epoch uint64, startHash, endHash uint32, status chash.MigrationStatus) {
	if b == nil || b.consHashRing == nil {
		return
	}
	b.consHashRing.UpdatePlanHintStatus(epoch, startHash, endHash, status)
}
