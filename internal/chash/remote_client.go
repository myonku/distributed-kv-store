package chash

import "context"

// ChashRemoteClient 是一致性哈希节点的远程客户端实现
type ChashRemoteClient struct {
	node *Node // 本节点信息
}

func NewChashRemoteClient(node *Node) *ChashRemoteClient {
	return &ChashRemoteClient{
		node: node,
	}
}

func (c *ChashRemoteClient) Put(ctx context.Context, nodeID, key, value string) error {
	return nil
}

func (c *ChashRemoteClient) Get(ctx context.Context, nodeID, key string) (string, error) {
	return "", nil
}

func (c *ChashRemoteClient) Delete(ctx context.Context, nodeID, key string) error {
	return nil
}
