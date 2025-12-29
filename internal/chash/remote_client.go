package chash

import (
	"context"
)

// TODO: 后续可注入 *http.Client、鉴权、重试策略等
type ChashRemoteClient struct {
}

func NewChashRemoteClient() *ChashRemoteClient {
	return &ChashRemoteClient{}
}

// Put 向指定节点存储键值对
func (c *ChashRemoteClient) Put(ctx context.Context, targetAddr, key, value string) error {
	return nil
}

// Get 从指定节点获取键值对
func (c *ChashRemoteClient) Get(ctx context.Context, targetAddr, key string) (string, error) {
	return "", nil
}

// Delete 从指定节点删除键值对
func (c *ChashRemoteClient) Delete(ctx context.Context, targetAddr, key string) error {
	return nil
}
