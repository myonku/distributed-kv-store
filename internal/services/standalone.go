package services

import (
	"context"
	"distributed-kv-store/internal/store"
)

// 单机模式下的 KVService 实现。
// 对外遵循 KVService 接口，对内委托给本地的 store.KVStore。
type StandaloneKVService struct {
	kv store.KVStore
}

func NewStandaloneKVService(kv store.KVStore) KVService {
	return &StandaloneKVService{kv: kv}
}

func (s *StandaloneKVService) Put(ctx context.Context, key, value string) error {
	return s.kv.Put(ctx, key, value)
}

func (s *StandaloneKVService) Get(ctx context.Context, key string) (string, error) {
	return s.kv.Get(ctx, key)
}

func (s *StandaloneKVService) Delete(ctx context.Context, key string) error {
	return s.kv.Delete(ctx, key)
}
