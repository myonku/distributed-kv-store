package chash

import "context"

// 用于远程转发调用的客户端实现，内部使用 HTTP 通信
type ChashRemoteClient struct {
	clients map[string]string // 节点ID->地址映射
}

func NewChashRemoteClient() *ChashRemoteClient {
	return &ChashRemoteClient{
		clients: make(map[string]string),
	}
}

// Put 向指定节点存储键值对
func (c *ChashRemoteClient) Put(ctx context.Context, nodeID, key, value string) error {
	return nil
}

// Get 从指定节点获取键值对
func (c *ChashRemoteClient) Get(ctx context.Context, nodeID, key string) (string, error) {
	return "", nil
}

// Delete 从指定节点删除键值对
func (c *ChashRemoteClient) Delete(ctx context.Context, nodeID, key string) error {
	return nil
}
