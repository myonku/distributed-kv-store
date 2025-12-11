package services

import (
	"context"
	"distributed-kv-store/internal/storage"
)

// 单机模式下的 KVService 实现。
type StandaloneKVService struct {
	st storage.Storage
}

func NewStandaloneKVService(st storage.Storage) KVService {
	return &StandaloneKVService{st: st}
}

func (s *StandaloneKVService) Put(ctx context.Context, key, value string) error {
	cmd := storage.Command{
		Op:    "put",
		Key:   key,
		Value: value,
	}
	_, err := s.st.AppendLog(ctx, cmd)
	return err
}

func (s *StandaloneKVService) Get(ctx context.Context, key string) (string, error) {
	return s.st.Get(ctx, key)
}

func (s *StandaloneKVService) Delete(ctx context.Context, key string) error {
	cmd := storage.Command{
		Op:  "delete",
		Key: key,
	}
	_, err := s.st.AppendLog(ctx, cmd)
	return err
}
