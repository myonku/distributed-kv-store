package gossip

import (
	"fmt"
	"strings"
)

// GossipMemberStatus 表示成员状态信息
type GossipMemberStatus struct {
	Member   Member
	IsSelf   bool
	StateStr string
}

// GossipClusterStatus 表示集群状态摘要
type GossipClusterStatus struct {
	Self    Member
	Members []GossipMemberStatus
}

// ClusterStatus 返回当前节点视图的成员状态
func (n *Node) ClusterStatus() GossipClusterStatus {
	if n == nil {
		return GossipClusterStatus{}
	}
	n.mu.RLock()
	defer n.mu.RUnlock()

	self := Member{}
	if n.self != nil {
		self = *n.self
	}

	members := make([]GossipMemberStatus, 0, len(n.members))
	for _, m := range n.members {
		if m == nil {
			continue
		}
		stateStr := "Unknown"
		switch m.State {
		case StateAlive:
			stateStr = "Alive"
		case StateSuspect:
			stateStr = "Suspect"
		case StateDead:
			stateStr = "Dead"
		}
		members = append(members, GossipMemberStatus{
			Member:   *m,
			IsSelf:   n.self != nil && m.ID == n.self.ID,
			StateStr: stateStr,
		})
	}

	return GossipClusterStatus{Self: self, Members: members}
}

// FormatClusterStatus 格式化输出集群状态信息（用于打印）
func (n *Node) FormatClusterStatus() string {
	status := n.ClusterStatus()
	var builder strings.Builder

	fmt.Fprintf(&builder, "Node ID: %s\n", status.Self.ID)
	fmt.Fprintf(&builder, "Members:\n")
	for _, member := range status.Members {
		selfMark := " "
		if member.IsSelf {
			selfMark = "*"
		}
		fmt.Fprintf(
			&builder,
			"  %s ID: %s, State: %s, Incarnation: %d, ClientAddr: %s, GossipAddr: %s, CHashAddr: %s, Weight: %d\n",
			selfMark,
			member.Member.ID,
			member.StateStr,
			member.Member.Incarnation,
			member.Member.ClientAddress,
			member.Member.GossipGRPCAddress,
			member.Member.CHashGRPCAddress,
			member.Member.Weight,
		)
	}
	return builder.String()
}
