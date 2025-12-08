package store

import "context"

// 对外部 Service 层更友好的封装，
// 基于底层 Storage（日志 + 状态机）实现简单的 KV 语义。
type KVStore interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// 单机模式下的 KVStore
type standaloneKVStore struct {
	storage Storage
}

func NewStandaloneKVStore(storage Storage) KVStore {
	return &standaloneKVStore{storage: storage}
}

// region
// 单机模型 KVStore 接口实现

func (s *standaloneKVStore) Put(ctx context.Context, key, value string) error {
	cmd := Command{Op: "set", Key: key, Value: value}
	index, err := s.storage.AppendLog(ctx, cmd)
	if err != nil {
		return err
	}
	return s.storage.ApplyLog(ctx, index)
}

func (s *standaloneKVStore) Get(ctx context.Context, key string) (string, error) {
	return s.storage.Get(ctx, key)
}

func (s *standaloneKVStore) Delete(ctx context.Context, key string) error {
	cmd := Command{Op: "delete", Key: key}
	index, err := s.storage.AppendLog(ctx, cmd)
	if err != nil {
		return err
	}
	return s.storage.ApplyLog(ctx, index)
}

// endregion
