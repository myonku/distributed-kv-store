package common

import (
	"context"
	"net/http"
)

// 用于业务转发，调用目标节点的对外 HTTP 接口
type RemoteClient interface {
	Put(ctx context.Context, targetAddr, key, value string) error
	Get(ctx context.Context, targetAddr, key string) (string, error)
	Delete(ctx context.Context, targetAddr, key string) error
}

// RemoteClient 实现远程请求转发客户端接口
type ChashRemoteClient struct {
	client *http.Client
}

func NewChashRemoteClient() RemoteClient {
	return &ChashRemoteClient{
		client: &http.Client{},
	}
}

// Put 向指定节点存储键值对
func (c *ChashRemoteClient) Put(ctx context.Context, targetAddr, key, value string) error {
	req, err := http.NewRequest(http.MethodPut, targetAddr+"/kv", nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	// 省略请求体和响应处理的具体实现
	return nil
}

// Get 从指定节点获取键值对
func (c *ChashRemoteClient) Get(ctx context.Context, targetAddr, key string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, targetAddr+"/kv?key="+key, nil)
	if err != nil {
		return "", err
	}
	req = req.WithContext(ctx)
	// 省略请求体和响应处理的具体实现
	return "", nil
}

// Delete 从指定节点删除键值对
func (c *ChashRemoteClient) Delete(ctx context.Context, targetAddr, key string) error {
	req, err := http.NewRequest(http.MethodDelete, targetAddr+"/kv?key="+key, nil)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	// 省略请求体和响应处理的具体实现
	return nil
}
