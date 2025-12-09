package services

import "context"

// 定义对外提供的 KV 存储服务接口,
// 不关心底层实现细节
type KVService interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}
