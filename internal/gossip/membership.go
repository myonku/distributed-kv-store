package gossip

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"time"
)

// 将一个新节点加入本地成员视图，并建立到该节点的 transport 连接
func (n *Node) AddMember(peer configs.ClusterNode) error {
	if n == nil {
		return errors.Error{Type: errors.ImportError, Info: "node not initialized"}
	}
	if peer.ID == "" {
		return errors.Error{Type: errors.InvalidArgument, Info: "invalid peer ID"}
	}
	if n.self != nil && peer.ID == n.self.ID {
		return errors.Error{Type: errors.InvalidArgument, Info: "cannot add self as member"}
	}

	// 先建立连接（生产实现可做重试/幂等）
	if n.transport != nil {
		if err := n.transport.AddPeer(peer); err != nil {
			return err
		}
	}

	now := time.Now().UnixNano()

	n.mu.Lock()
	defer n.mu.Unlock()

	old, existed := n.members[peer.ID]
	var oldSnapshot Member
	if existed && old != nil {
		oldSnapshot = *old
	}

	m := &Member{
		ID:                peer.ID,
		GossipGRPCAddress: peer.GossipGRPCAddress,
		CHashGRPCAddress:  peer.CHashGRPCAddress,
		ClientAddress:     peer.ClientAddress,
		Weight:            peer.Weight,
		State:             StateAlive,
		// Incarnation：由节点自己递增/传播；这里占位为保持原值或 0
		Incarnation:  0,
		StateUpdated: now,
	}
	if existed && old != nil {
		// 保留更大的 incarnation（避免意外回退）
		if old.Incarnation > m.Incarnation {
			m.Incarnation = old.Incarnation
		}
	}

	n.members[peer.ID] = m

	if oldSnapshot.ID == "" {
		n.emitEventIfChangedLocked(n.ctx, *m, Member{})
	} else {
		n.emitEventIfChangedLocked(n.ctx, *m, oldSnapshot)
	}
	return nil
}

// 将成员标记为 Dead，并移除 transport 连接
func (n *Node) RemoveMember(peerID string) error {
	if n == nil {
		return errors.Error{Type: errors.ImportError, Info: "node not initialized"}
	}
	if peerID == "" {
		return errors.Error{Type: errors.InvalidArgument, Info: "invalid peer ID"}
	}
	if n.self != nil && peerID == n.self.ID {
		return errors.Error{Type: errors.InvalidArgument, Info: "cannot remove self as member"}
	}

	now := time.Now().UnixNano()

	n.mu.Lock()
	m, ok := n.members[peerID]
	if !ok || m == nil {
		n.mu.Unlock()
		return nil
	}
	old := *m
	m.State = StateDead
	m.StateUpdated = now
	// 触发成员视图变更事件，由后续的 reapLoop 真正移除成员
	n.emitEventIfChangedLocked(n.ctx, *m, old)
	n.mu.Unlock()

	if n.transport != nil {
		_ = n.transport.RemovePeer(peerID)
	}
	return nil
}
