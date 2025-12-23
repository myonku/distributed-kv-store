package gossip

import "context"

// Failure detector loop

// 选取探测目标并发送 Ping 消息
func (n *Node) probeOnce(ctx context.Context) {

}

// 处理探测超时
func (n *Node) onProbeTimeout(ctx context.Context, peerID string) {

}

// 清理状态为 Suspect 和 Dead 的节点
func (n *Node) reapSuspectsAndDeads(ctx context.Context) {

}

// Gossip dissemination loop

// 选取 Gossip 目标并发送 PushPull 消息
func (n *Node) gossipOnce(ctx context.Context) {

}

// 生成本地节点的 Digest 列表
func (n *Node) makeDigest(ctx context.Context) {

}

// 应用来自 PushPull 响应的 Delta 更新
func (n *Node) applyDelta(ctx context.Context, delta []Member) {

}

// 成员视图发生变化时触发事件
func (n *Node) emitEventIfChanged(ctx context.Context, member Member, oldState NodeState) {

}
