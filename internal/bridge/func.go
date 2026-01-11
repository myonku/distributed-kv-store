package bridge

import (
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
)

func (b *MemberBridge) SelfID() string {
	if b == nil || b.gossipNode == nil {
		return ""
	}
	return b.gossipNode.SelfID()
}

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
		return "", false, errors.ErrResourceNotInit
	}
	return b.consHashRing.GetNode(key)
}

// 根据 key 查询负责节点 ID 列表
func (b *MemberBridge) OwnerNodeIDs(key string) (nodeIDs []string, err error) {
	if b == nil || b.consHashRing == nil {
		return []string{}, errors.ErrResourceNotInit
	}
	return b.consHashRing.GetNodes(key)
}

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
