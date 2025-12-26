package gossip

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"math/rand/v2"
	"time"
)

// 启动时引导同步（seed 节点列表来自配置）
func (n *Node) Join(seeds []configs.ClusterNode) error {

	if n == nil {
		return errors.ErrResourceNotInit
	}

	// 初始化本地视图
	n.mu.Lock()
	if n.self != nil {
		if _, ok := n.members[n.self.ID]; !ok {
			m := *n.self
			n.members[m.ID] = &m
		}
	}
	n.mu.Unlock()

	if n.transport == nil {
		return nil
	}

	// 连接种子节点
	var firstSeedID string
	for _, s := range seeds {
		if n.self != nil && s.ID == n.self.ID {
			continue
		}
		if err := n.transport.AddPeer(s); err != nil {
			return err
		}
		if firstSeedID == "" {
			firstSeedID = s.ID
		}

		// 本地也放一个占位成员（地址/权重来自配置），真实状态后续由 gossip 合并驱动
		n.mu.Lock()
		if _, ok := n.members[s.ID]; !ok {
			n.members[s.ID] = &Member{
				ID:                s.ID,
				GossipGRPCAddress: s.GossipGRPCAddress,
				ClientAddress:     s.ClientAddress,
				Weight:            s.Weight,
				State:             StateAlive,
				Incarnation:       0,
				StateUpdated:      time.Now().UnixNano(),
			}
		}
		n.mu.Unlock()
	}

	// 没有种子节点则直接返回
	if firstSeedID == "" {
		return nil
	}

	// 引导同步：对 seed 做一次 PushPull
	baseCtx := n.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	ctx := baseCtx
	var cancel context.CancelFunc
	if n.probeTimeout > 0 {
		ctx, cancel = context.WithTimeout(baseCtx, n.probeTimeout)
		defer cancel()
	}

	resp, err := n.transport.PushPull(ctx, firstSeedID, &PushPullRequest{
		FromID:      n.self.ID,
		Digests:     n.makeDigest(),
		FullMembers: n.Snapshot(),
	})
	if err != nil {
		return err
	}
	if resp != nil {
		n.applyDelta(baseCtx, resp.Delta)
	}
	return nil
}

// 获取节点快照
func (n *Node) Snapshot() []Member {
	n.mu.RLock()
	defer n.mu.RUnlock()
	snapshot := make([]Member, 0, len(n.members))
	for _, m := range n.members {
		snapshot = append(snapshot, *m)
	}
	return snapshot
}

// 生成本地节点的 Digest 列表
func (n *Node) makeDigest() []Digest {
	n.mu.RLock()
	defer n.mu.RUnlock()
	digests := make([]Digest, 0, len(n.members))
	for _, m := range n.members {
		digests = append(digests, Digest{
			ID:          m.ID,
			Incarnation: m.Incarnation,
			State:       m.State,
		})
	}
	return digests
}

// 订阅节点事件，提供事件广播通道
func (n *Node) Subscribe() <-chan Event {
	// 占位：目前直接复用共享 events 通道。
	// TODO: 如需多订阅者独立消费，可在这里实现 fan-out。
	return n.events
}

// 事件通道。仅提供单一只通道供外部消费
func (n *Node) Events() <-chan Event {
	return n.events
}

// 随机选取探测目标并发送 Ping 消息
func (n *Node) probeOnce(ctx context.Context) error {

	if n == nil || n.transport == nil || n.self == nil {
		return errors.ErrResourceNotInit
	}
	if ctx == nil {
		ctx = context.Background()
	}

	peerID, err := n.pickOnePeerID()
	if err != nil {
		return err
	}

	rpcCtx := ctx
	var cancel context.CancelFunc
	if n.probeTimeout > 0 {
		rpcCtx, cancel = context.WithTimeout(ctx, n.probeTimeout)
		defer cancel()
	}

	resp, err := n.transport.Ping(rpcCtx, peerID, &PingRequest{
		FromID:          n.self.ID,
		FromIncarnation: n.self.Incarnation,
	})
	if err != nil || resp == nil || !resp.OK {
		n.onProbeTimeout(ctx, peerID)
		return nil
	}

	// 成功：刷新本地视图（这里不做 incarnation 逻辑，只做占位刷新）
	n.mu.Lock()
	defer n.mu.Unlock()
	if m, ok := n.members[peerID]; ok {
		old := *m
		m.State = StateAlive
		m.StateUpdated = time.Now().UnixNano()
		n.emitEventIfChangedLocked(ctx, *m, old)
	}
	return nil
}

// 处理探测超时
func (n *Node) onProbeTimeout(ctx context.Context, peerID string) {

	if ctx == nil {
		ctx = context.Background()
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	m, ok := n.members[peerID]
	if !ok {
		return
	}
	// 已经是 Dead 则忽略
	if m.State == StateDead {
		return
	}
	old := *m
	// TODO: 更精细的失败检测策略（阈值、间接探测、反熵等）
	m.State = StateSuspect
	m.StateUpdated = time.Now().UnixNano()
	n.emitEventIfChangedLocked(ctx, *m, old)
}

// 清理状态为 Suspect 和 Dead 的节点
func (n *Node) reapSuspectsAndDeads(ctx context.Context) {

	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UnixNano()
	var toRemove []string

	n.mu.Lock()
	for id, m := range n.members {
		if n.self != nil && id == n.self.ID {
			continue
		}
		age := time.Duration(now - m.StateUpdated)
		switch m.State {
		case StateSuspect:
			// 超时则标记为 Dead
			if n.suspectTimeout > 0 && age >= n.suspectTimeout {
				old := *m
				m.State = StateDead
				m.StateUpdated = now
				n.emitEventIfChangedLocked(ctx, *m, old)
			}
		case StateDead:
			// 超时则移除
			if n.deadTimeout > 0 && age >= n.deadTimeout {
				toRemove = append(toRemove, id)
			}
		}
	}
	for _, id := range toRemove {
		delete(n.members, id)
	}
	n.mu.Unlock()

	// 移除 transport 中的 Peer 连接
	if n.transport != nil {
		for _, id := range toRemove {
			_ = n.transport.RemovePeer(id)
		}
	}
}

// 选取 Gossip 目标并发送 PushPull 消息（fanout）
func (n *Node) gossipOnce(ctx context.Context) error {

	if n == nil || n.transport == nil || n.self == nil {
		return errors.ErrResourceNotInit
	}
	if ctx == nil {
		ctx = context.Background()
	}

	fanout := n.fanout
	if fanout <= 0 {
		fanout = 1
	}
	// 选取 fanout 个目标（Alive/Suspect，跳过 Dead/self）
	peers, err := n.pickPeers(fanout)
	// 没有可用 Peer 则跳过
	if len(peers) == 0 || err != nil {
		return nil
	}

	digests := n.makeDigest()
	full := n.Snapshot()

	for _, peerID := range peers {
		rpcCtx := ctx
		var cancel context.CancelFunc
		if n.probeTimeout > 0 {
			rpcCtx, cancel = context.WithTimeout(ctx, n.probeTimeout)
			defer cancel()
		}
		// 发送 PushPull 请求
		resp, err := n.transport.PushPull(rpcCtx, peerID, &PushPullRequest{
			FromID:      n.self.ID,
			Digests:     digests,
			FullMembers: full,
		})
		// 忽略错误，继续下一个
		if err != nil || resp == nil {
			continue
		}
		n.applyDelta(ctx, resp.Delta)
	}
	return nil
}

// 应用来自 PushPull 响应的 Delta 更新
func (n *Node) applyDelta(ctx context.Context, delta []Member) error {
	if n == nil || len(delta) == 0 {
		return errors.ErrResourceNotInit
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now().UnixNano()
	n.mu.Lock()
	defer n.mu.Unlock()

	for i := range delta {
		incoming := delta[i]
		if incoming.ID == "" {
			continue
		}
		// 忽略自身信息
		if n.self != nil && incoming.ID == n.self.ID {
			continue
		}

		local, ok := n.members[incoming.ID]
		if !ok {
			m := incoming
			if m.StateUpdated == 0 {
				m.StateUpdated = now
			}
			n.members[m.ID] = &m
			n.emitEventIfChangedLocked(ctx, m, Member{})
			continue
		}

		// 占位冲突规则：incarnation 大者胜；相等则按 stateUpdated 较新者胜
		shouldApply := incoming.Incarnation > local.Incarnation ||
			(incoming.Incarnation == local.Incarnation && incoming.StateUpdated > local.StateUpdated)
		if !shouldApply {
			continue
		}

		old := *local
		*local = incoming
		if local.StateUpdated == 0 {
			local.StateUpdated = now
		}
		n.emitEventIfChangedLocked(ctx, *local, old)
	}
	return nil
}

// 成员视图发生变化时触发事件
func (n *Node) emitEventIfChanged(ctx context.Context, member Member, old Member) {

	if ctx == nil {
		ctx = context.Background()
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.emitEventIfChangedLocked(ctx, member, old)
}

// 更新成员视图发生变化时触发事件（调用方需持有锁）

func (n *Node) emitEventIfChangedLocked(_ context.Context, member Member, old Member) {

	// 状态变化或关键字段更新（incarnation/stateUpdated/地址/权重等）时触发事件
	changed := false
	if old.ID == "" {
		changed = true
	} else if member.State != old.State ||
		member.Incarnation != old.Incarnation ||
		member.StateUpdated != old.StateUpdated ||
		member.GossipGRPCAddress != old.GossipGRPCAddress ||
		member.ClientAddress != old.ClientAddress ||
		member.Weight != old.Weight {
		changed = true
	}
	if !changed {
		return
	}

	var et EventType
	if old.ID != "" && member.State == old.State {
		// 状态没变但重要字段变了：用 MembershipChanged 便于外部（如 ring）刷新
		et = EventMembershipChanged
	} else {
		switch member.State {
		case StateAlive:
			et = EventMemberUp
		case StateSuspect:
			et = EventMemberSuspect
		case StateDead:
			et = EventMemberDead
		default:
			et = EventMembershipChanged
		}
	}

	// 快照（避免调用 Snapshot() 造成锁重入）
	snapshot := make([]Member, 0, len(n.members))
	for _, m := range n.members {
		snapshot = append(snapshot, *m)
	}

	// 非阻塞发送事件
	select {
	case n.events <- Event{Type: et, Member: member, Snapshot: snapshot}:
	default:
		// TODO: events 满时的策略（丢弃/扩容/告警）
	}
}

// 从当前 members 中随机挑一个可用 peer
func (n *Node) pickOnePeerID() (string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var candidates []string
	for id, m := range n.members {
		if n.self != nil && id == n.self.ID {
			continue
		}
		if m.State == StateDead {
			continue
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		return "", errors.ErrNoAvailablePeer
	}
	return candidates[rand.IntN(len(candidates))], nil
}

// 随机挑选 fanout 个 peer（不含 self/dead）
func (n *Node) pickPeers(fanout int) ([]string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if fanout <= 0 {
		return nil, errors.ErrInvalidArgument
	}
	var candidates []string
	for id, m := range n.members {
		if n.self != nil && id == n.self.ID {
			continue
		}
		if m.State == StateDead {
			continue
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		return nil, errors.ErrNoAvailablePeer
	}

	for i := len(candidates) - 1; i > 0; i-- {
		j := rand.IntN(i + 1)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}
	if fanout >= len(candidates) {
		return candidates, errors.ErrNoAvailablePeer
	}
	return candidates[:fanout], nil
}
