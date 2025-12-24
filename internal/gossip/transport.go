package gossip

import (
	"context"
	"distributed-kv-store/configs"
	"time"
)

type Transport interface {
	// 发送 Ping 消息
	Ping(ctx context.Context, to string, req *PingRequest) (*PingResponse, error)
	// 发送 PushPull 消息
	PushPull(ctx context.Context, to string, req *PushPullRequest) (*PushPullResponse, error)
	// 添加新的集群节点连接（本地）
	AddPeer(peer configs.ClusterNode) error
	// 移除某个集群节点的连接（本地）
	RemovePeer(peerID string) error
}

// Ping 请求与响应

type PingRequest struct {
	FromID          string
	FromIncarnation uint64
}

type PingResponse struct {
	OK bool
}

// PushPull 请求与响应

type PushPullRequest struct {
	FromID      string
	Digests     []Digest
	FullMembers []Member
}

type PushPullResponse struct {
	Delta []Member // 需要更新的成员信息
}

// 处理来自其他节点的 Ping（由 RPC 层调用）
func (n *Node) HandlePing(ctx context.Context, req *PingRequest) (*PingResponse, error) {

	if req == nil {
		return &PingResponse{OK: false}, nil
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// 校验请求
	fromID := req.FromID
	if fromID == "" || fromID == n.self.ID {
		return &PingResponse{OK: false}, nil
	}

	now := time.Now().UnixNano()
	member, ok := n.members[fromID]
	if !ok {
		// 不允许通过 Ping 自动引入成员，忽略该请求
		// 后续可能更新 Ping 消息内容支持引入新成员
		// m := &Member{ID: fromID, State: StateAlive, Incarnation: req.FromIncarnation, StateUpdated: now}
		// n.members[fromID] = m
		// n.emitEventIfChanged(ctx, *m, StateDead)
		return &PingResponse{OK: true}, nil
	}

	oldState := member.State
	// 占位合并规则：incarnation 更大则覆盖；否则仅刷新存活时间
	if req.FromIncarnation > member.Incarnation {
		member.Incarnation = req.FromIncarnation
		member.State = StateAlive
		member.StateUpdated = now
	} else {
		// 更新存活时间
		member.StateUpdated = now
		// 保证状态为存活
		if member.State != StateAlive {
			member.State = StateAlive
		}
	}

	n.emitEventIfChanged(ctx, *member, oldState)

	return &PingResponse{OK: true}, nil
}

// 处理来自其他节点的 PushPull（由 RPC 层调用）
func (n *Node) HandlePushPull(ctx context.Context, req *PushPullRequest) (*PushPullResponse, error) {

	if req == nil {
		return &PushPullResponse{Delta: []Member{}}, nil
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now().UnixNano()

	// 合并 FullMembers
	for i := range req.FullMembers {
		incoming := req.FullMembers[i]
		// 忽略无效或自身信息
		if incoming.ID == "" || n.self != nil && incoming.ID == n.self.ID {
			continue
		}

		local, ok := n.members[incoming.ID]
		if !ok {
			m := incoming // copy
			// 如果对端未携带时间，至少保证本地有更新时间
			if m.StateUpdated == 0 {
				m.StateUpdated = now
			}
			n.members[incoming.ID] = &m
			continue
		}

		// 占位冲突规则：incarnation 大者胜；相等则按 stateUpdated 较新者胜
		if incoming.Incarnation > local.Incarnation ||
			(incoming.Incarnation == local.Incarnation && incoming.StateUpdated > local.StateUpdated) {
			oldState := local.State
			*local = incoming
			if local.StateUpdated == 0 {
				local.StateUpdated = now
			}
			n.emitEventIfChanged(ctx, *local, oldState)
		}
	}

	// 基于 Digests 计算 Delta
	remote := make(map[string]Digest, len(req.Digests))
	for _, d := range req.Digests {
		if d.ID == "" {
			continue
		}
		remote[d.ID] = d
	}

	delta := make([]Member, 0)
	for id, local := range n.members {
		// 忽略自身信息
		if n.self != nil && id == n.self.ID {
			continue
		}

		rd, ok := remote[id]
		if !ok {
			// 对端没有该成员，回传完整信息
			delta = append(delta, *local)
			continue
		}
		// 对端 incarnation 更小/状态更旧，回传
		if local.Incarnation > rd.Incarnation {
			delta = append(delta, *local)
			continue
		}
		// 如果 incarnation 相同但 state 为更旧，回传
		if local.Incarnation == rd.Incarnation && local.State > rd.State {
			delta = append(delta, *local)
			continue
		}
	}

	return &PushPullResponse{Delta: delta}, nil
}
