package gossip

import "distributed-kv-store/configs"

// 启动时引导同步（seed 节点列表来自配置）
func (n *Node) Join(seeds []configs.ClusterNode) error {
	return nil
}

// 获取节点快照
func (n *Node) Snapshot() []Member {
	return nil
}

// 订阅节点事件
func (n *Node) Subscribe() <-chan Event {
	return nil
}

// 事件通道
func (n *Node) Events() <-chan Event {
	return n.events
}
