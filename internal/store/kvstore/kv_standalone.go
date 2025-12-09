package kvstore

import (
	"context"
	"distributed-kv-store/internal/store"
)

// 单机模式下的 KVStore
type standaloneKVStore struct {
	storage store.Storage
}

func NewStandaloneKVStore(storage store.Storage) store.KVStore {
	return &standaloneKVStore{storage: storage}
}

func (s *standaloneKVStore) Put(ctx context.Context, key, value string) error {
	cmd := store.Command{Op: "set", Key: key, Value: value}
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
	cmd := store.Command{Op: "delete", Key: key}
	index, err := s.storage.AppendLog(ctx, cmd)
	if err != nil {
		return err
	}
	return s.storage.ApplyLog(ctx, index)
}
